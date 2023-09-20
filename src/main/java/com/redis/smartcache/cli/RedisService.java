package com.redis.smartcache.cli;

import java.util.List;

import com.redis.smartcache.cli.structures.QueryInfo;
import com.redis.smartcache.cli.structures.TableInfo;
import com.redis.smartcache.core.RuleConfig;

public interface RedisService {
    List<QueryInfo> getQueries();
    void commitRules(List<RuleConfig> rules);
    List<RuleConfig> getRules();
    List<TableInfo> getTables();
    void clearMetrics();
}
