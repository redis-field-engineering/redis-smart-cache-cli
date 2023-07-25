package com.redis.smartcache.cli;

import static com.redis.smartcache.core.RuleSessionManager.KEY_CONFIG;

import java.io.IOException;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Properties;

import com.fasterxml.jackson.dataformat.javaprop.JavaPropsMapper;
import com.redis.lettucemod.api.StatefulRedisModulesConnection;
import com.redis.lettucemod.search.AggregateOptions;
import com.redis.lettucemod.search.AggregateResults;
import com.redis.lettucemod.search.Apply;
import com.redis.lettucemod.search.Document;
import com.redis.lettucemod.search.Group;
import com.redis.lettucemod.search.Reducer;
import com.redis.lettucemod.search.Reducers;
import com.redis.lettucemod.search.SearchResults;
import com.redis.lettucemod.timeseries.TimeRange;
import com.redis.smartcache.cli.structures.QueryInfo;
import com.redis.smartcache.cli.structures.TableInfo;
import com.redis.smartcache.core.ClientManager;
import com.redis.smartcache.core.Config;
import com.redis.smartcache.core.KeyBuilder;
import com.redis.smartcache.core.Mappers;
import com.redis.smartcache.core.RuleConfig;
import com.redis.smartcache.core.RulesetConfig;
import com.redis.smartcache.core.StreamConfigManager;

import io.airlift.units.Duration;

//@Service
public class RedisServiceImpl implements RedisService{
    Config conf;

    ClientManager manager;

    StatefulRedisModulesConnection<String, String> connection;

    StreamConfigManager<RulesetConfig> configManager;

    private final JavaPropsMapper mapper = Mappers.propsMapper();

    public RedisServiceImpl(RedisConfig config){
        conf = config.conf();
        manager = config.abstractRedisClient();
        connection = config.modClient();
        RulesetConfig ruleset = conf.getRuleset();
        String key = KeyBuilder.of(conf).build(KEY_CONFIG);
        configManager = new StreamConfigManager<>(manager.getClient(conf), key, ruleset, mapper);
        try{
            configManager.start();
        } catch (IOException e){
            throw new IllegalStateException("Could not start Redis Service", e);
        }
    }


    public String ping(){
        return connection.sync().ping();
    }

    public List<RuleConfig> getRules(){
        RulesetConfig ruleset = conf.getRuleset();

        return Arrays.asList(ruleset.getRules());
    }

    static String configKeyName(String applicationName){
        return String.format("%s:config", applicationName);
    }

    static String HashKeyName(String applicationName, String id){
        return String.format("%s:query:%s", applicationName, id);

    }

    static String IndexName(String applicationName){
        return String.format("%s-query-idx", applicationName);
    }

    public List<QueryInfo> getQueries(){
        List<QueryInfo> response = new ArrayList<>();
        List<RuleConfig> rules = getRules();

        SearchResults<String, String> searchResults = connection.sync().ftSearch(IndexName(conf.getName()), "*");

        for(Document<String, String> doc : searchResults){

            QueryInfo qi = QueryInfo.fromDocument(doc);
            Optional<RuleConfig> currentRule = QueryInfo.matchRule(qi.getQuery(), rules);
            currentRule.ifPresent(qi::setCurrentRule);
            response.add(qi);

        }
        return response;
    }

    public void commitRules(List<RuleConfig> rules){
        Map<String, List<RuleConfig>> map = new HashMap<>();
        if(rules.isEmpty()){
            RuleConfig defaultRule = new RuleConfig();
            defaultRule.setTtl(Duration.valueOf("0s"));
            map.put("rules", Collections.singletonList(defaultRule));
        }
        else{
            map.put("rules",rules);
        }

        JavaPropsMapper mapper = Mappers.propsMapper();
        try{
            Properties props = mapper.writeValueAsProperties(map);
            List<String> listArgs = new ArrayList<>();

            for(Object o : props.keySet().stream().sorted().toList()){
                listArgs.add((String)o);
                listArgs.add((String)props.get(o));
            }

            String key = KeyBuilder.of(conf).build(KEY_CONFIG);
            connection.sync().xadd(key,listArgs.toArray());
        } catch (IOException ignored){

        }
    }

    public List<TableInfo> getTables(){

        List<RuleConfig> rules = getRules();
        List<TableInfo> tableInfos = new ArrayList<>();
        String[] groupStrs = {"name"};
        Reducer[] reducers = {new Reducers.Sum.Builder("count").as("accessFrequency").build(), new Reducers.Avg.Builder("mean").as("avgQueryTime").build()};
        AggregateOptions<String,String> options = AggregateOptions.<String,String>builder().operation(new Apply<String,String>("split(@table, ',')", "name")).operation(new Group(groupStrs, reducers)).build();
        AggregateResults<String> res = connection.sync().ftAggregate(IndexName(conf.getName()), "*", options);
        for(Map<String,Object> item : res){
            String name = item.get("name").toString();
            double avgQueryTime = Double.parseDouble(item.get("avgQueryTime").toString());
            long accessFrequency = Long.parseLong(item.get("accessFrequency").toString());
            Optional<RuleConfig> rule = rules.stream().filter(x->x.getTablesAny() != null && x.getTablesAny().contains(name)).findAny();
            TableInfo.Builder builder = new TableInfo.Builder().name(name).accessFrequency(accessFrequency).queryTime(avgQueryTime);
            rule.ifPresent(builder::rule);

            tableInfos.add(builder.build());
        }

        return tableInfos;
    }

    @SuppressWarnings("unchecked")
    public void clearMetrics(){
        String[] groups = new String[0];
        Reducer[] reducers = { new Reducers.ToList.Builder("id").as("id").build() };
        AggregateOptions<String,String> options = AggregateOptions.<String,String>builder().operation(new Group(groups,reducers)).build();
        AggregateResults<String> res = connection.sync().ftAggregate(IndexName(conf.getName()),"*", options);
        if(res.size() < 1){
            return;
        }

        List<String> ids = (List<String>) res.get(0).get("id");
        for(String id : ids){
            List<String> keys = connection.sync().tsQueryIndex(String.format("id=%s",id));
            for(String key : keys){
                connection.sync().tsDel(key, TimeRange.from(0).to(Long.MAX_VALUE).build());
            }
        }
    }
}
