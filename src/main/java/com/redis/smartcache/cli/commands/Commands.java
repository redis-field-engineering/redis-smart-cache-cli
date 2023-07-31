package com.redis.smartcache.cli.commands;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.Set;
import java.util.UUID;
import java.util.stream.Collectors;

import org.jline.utils.InfoCmp;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.shell.component.ConfirmationInput;
import org.springframework.shell.component.StringInput;
import org.springframework.shell.component.flow.ComponentFlow;
import org.springframework.shell.component.support.SelectorItem;
import org.springframework.shell.standard.AbstractShellComponent;
import org.springframework.shell.standard.ShellComponent;
import org.springframework.shell.standard.ShellMethod;
import org.springframework.shell.standard.ShellOption;

import com.redis.smartcache.cli.RedisConfig;
import com.redis.smartcache.cli.RedisService;
import com.redis.smartcache.cli.RedisServiceImpl;
import com.redis.smartcache.cli.components.ConfirmationInputExtension;
import com.redis.smartcache.cli.components.StringInputExtension;
import com.redis.smartcache.cli.components.TableSelector;
import com.redis.smartcache.cli.structures.Action;
import com.redis.smartcache.cli.structures.QueryInfo;
import com.redis.smartcache.cli.structures.RuleInfo;
import com.redis.smartcache.cli.structures.RuleType;
import com.redis.smartcache.cli.structures.RuleTypeInfo;
import com.redis.smartcache.cli.structures.SortDirection;
import com.redis.smartcache.cli.structures.SortField;
import com.redis.smartcache.cli.structures.TableInfo;
import com.redis.smartcache.cli.util.Util;
import com.redis.smartcache.core.RuleConfig;

import io.airlift.units.Duration;

@ShellComponent
public class Commands extends AbstractShellComponent {
    final String LIST_APPLICATION_QUERIES = "List application queries";
    final String CREATE_RULE = "Create query caching rule";
    final String LIST_TABLES = "List database tables";
    final String LIST_RULES = "List query caching rules";
    final String EXIT = "Exit";
    final String RESET_CONFIG = "Reset smart cache configuration";
    final String CLEAR_METRICS = "Clear smart cache metrics";

    final String tableInstructions = "Press [ENTER] to edit\nPress 'c' to commit\nPress [ESC] to go back\nPress [CTRL+C] to exit\n\n";

    @Autowired
    private ComponentFlow.Builder componentFlowBuilder;


    public Optional<RuleTypeInfo> getRuleType(){
        List<SelectorItem<RuleTypeInfo>> ruleTypes = Arrays.asList(
                SelectorItem.of(RuleType.TABLES.getValue(), new RuleTypeInfo(RuleType.TABLES,"Enter a comma-separated list of tables to match against:")),
                SelectorItem.of(RuleType.TABLES_ALL.getValue(), new RuleTypeInfo(RuleType.TABLES_ALL,"Enter a comma-separated list of tables to match against:")),
                SelectorItem.of(RuleType.TABLES_ANY.getValue(), new RuleTypeInfo(RuleType.TABLES_ANY,"Enter a comma-separated list of tables to match against:")),
                SelectorItem.of(RuleType.QUERY_IDS.getValue(), new RuleTypeInfo(RuleType.QUERY_IDS,"Enter a comma-separated list of Query IDs to match against:")),
                SelectorItem.of(RuleType.REGEX.getValue(), new RuleTypeInfo(RuleType.REGEX,"Enter a regular expression to match against:"))
        );

        TableSelector<RuleTypeInfo, SelectorItem<RuleTypeInfo>> component = new TableSelector<>(getTerminal(),
                ruleTypes, "rules", null, "Select Rule Type", true, 1, "");
        component.setResourceLoader(getResourceLoader());
        component.setTemplateExecutor(getTemplateExecutor());
        TableSelector.SingleItemSelectorContext<RuleTypeInfo, SelectorItem<RuleTypeInfo>> context = component
                .run(TableSelector.SingleItemSelectorContext.empty(1, ""));
        if(!component.isEscapeMode() && context.getResultItem().isPresent()){
            return Optional.of(context.getResultItem().get().getItem());
        }

        return Optional.empty();
    }

    public Optional<Duration> getTtl(String message){
        boolean displayError = false;
        while(true){
            String prompt;
            if(message.isEmpty()){
                prompt = "Enter a TTL in the form of a duration (e.g., 1h, 300s, 5m)";
            }
            else{
                prompt = String.format("%s%nEnter a TTL in the form of a duration (e.g., 1h, 300s, 5m)",message);
            }

            prompt += displayError?" - Duration must be a number plus a time unit (e.g., 1h, 3002, 5m). Please try again:" : ":";

            StringInputExtension stringInputComponent = new StringInputExtension(getTerminal(), prompt,"30m");
            stringInputComponent.setResourceLoader(getResourceLoader());
            stringInputComponent.setTemplateExecutor(getTemplateExecutor());
            StringInput.StringInputContext stringInputContext = stringInputComponent.run(StringInput.StringInputContext.empty());
            if(stringInputComponent.isEscapeMode()){
                return Optional.empty();
            }

            try{
                return Optional.of(Duration.valueOf(stringInputContext.getResultValue()));
            }
            catch (IllegalArgumentException e){
                displayError = true;
            }
        }


    }

    public Optional<String> getMatch(RuleTypeInfo ruleType){
        StringInputExtension component = new StringInputExtension(getTerminal(),ruleType.getMessage(),"");
        component.setResourceLoader(getResourceLoader());
        component.setTemplateExecutor(getTemplateExecutor());
        StringInput.StringInputContext context = component.run(StringInput.StringInputContext.empty());
        if(component.isEscapeMode()){
            return Optional.empty();
        }

        return Optional.of(context.getResultValue());
    }

    public Optional<Boolean> getConfirmation(RuleInfo info){
        String prompt = String.format("Rule Type: %s, Rule Match: %s, Rule TTL: %s", info.ruleType(), info.ruleMatch(), info.getRule().getTtl());
        ConfirmationInputExtension component = new ConfirmationInputExtension(getTerminal(), prompt, false);
        component.setResourceLoader(getResourceLoader());
        component.setTemplateExecutor(getTemplateExecutor());
        ConfirmationInput.ConfirmationInputContext context = component.run(ConfirmationInput.ConfirmationInputContext.empty());
        if(component.isEscapeMode()){
            return Optional.empty();
        }

        return Optional.of(context.getResultValue());
    }

    public Optional <RuleConfig> newRuleDialogCustom(boolean confirm){
        Optional<RuleTypeInfo> ruleType;
        Optional<String> match;
        Optional<Boolean> confirmed = Optional.empty();
        Optional<Duration> ttl = Optional.empty();
        Optional<RuleConfig> rule = Optional.empty();
        do{
            ruleType = getRuleType();
            if(!ruleType.isPresent()){
                break;
            }

            do{
                match = getMatch(ruleType.get());
                if(!match.isPresent()){
                    break;
                }

                ttl = getTtl("");
            } while(!ttl.isPresent());

            if(!match.isPresent()){
                continue;
            }

            rule = Optional.of(Util.createRule(ruleType.get().getType(), match.get(),ttl.get()));

            if(confirm){
                confirmed = getConfirmation(new RuleInfo(rule.get(), RuleInfo.Status.New));
                if(confirmed.isPresent() && !confirmed.get()){
                    break;
                }
            }

        }while(!confirmed.isPresent() && confirm);

        if((confirmed.isPresent() && confirmed.get()) || !confirm){
            return rule;
        }

        return Optional.empty();
    }

    public void createRule(RedisService client){
        Optional<RuleConfig> newRule = newRuleDialogCustom(true);
        if(newRule.isPresent()){
            List<RuleConfig> rules = new ArrayList<>(client.getRules());
            rules.add(0, newRule.get());
            client.commitRules(rules);
        }
    }

    /**
     * There does not appear to be any means in Spring Shell to autowire configuration parameters,
     * hence we need to initialize the client in each individual command, this is fine since we'll just pass the client
     * around in the interactive command and the non-interactive commands are run ad-hoc.
     * @param host the Redis host
     * @param port the Redis port
     * @return a connected RedisService
     */
    private static RedisService initializeClient(String host, String port, String applicationName){
        RedisConfig config = new RedisConfig(host,port, applicationName);
        return new RedisServiceImpl(config);
    }

    @ShellMethod(key = "list-queries")
    public String listQueries(
            @ShellOption(value = {"-n","--hostname"}, defaultValue = "localhost")String host,
            @ShellOption(value = {"-p","--port"}, defaultValue = "6379") String port,
            @ShellOption(value = {"-s","--application-name"}, defaultValue = "smartcache") String applicationName,
            @ShellOption(value = {"-d","--sort-direction"}, defaultValue = "desc") String sortDirectionStr,
            @ShellOption(value = {"-b","--sort-by"}, defaultValue = "query-time") String sortByStr
    ){
        SortDirection sortDirection;
        SortField sortBy;
        try{
            sortDirection = SortDirection.valueOf(sortDirectionStr.toLowerCase());
        }
        catch (IllegalArgumentException e){
            return String.format("Invalid Sort Direction %s", sortDirectionStr);
        }

        try{
            sortBy = SortField.valueOfOverride(sortByStr.toLowerCase());
        }
        catch(IllegalArgumentException e){
            return String.format("Invalid Sort By: %s", sortByStr);
        }


        RedisService client = initializeClient(host, port, applicationName);
        StringBuilder sb = new StringBuilder();
        sb.append("\n");
        List<QueryInfo> queries = client.getQueries();

        queries.sort((first,second)->QueryInfo.compare(first,second,sortDirection,sortBy));

        int columnWidth = (getTerminal().getWidth()-10)/8;
        sb.append(String.format("%s%n",QueryInfo.getHeaderRow(columnWidth, false)));
        for(QueryInfo qi : queries){
            sb.append(String.format("%s%n",qi.toRowString(columnWidth, false)));
        }
        return sb.toString();
    }

    @ShellMethod(key = "make-rule")
    public String makeRule(
            @ShellOption(value = {"-n","--hostname"}, defaultValue = "localhost")String host,
            @ShellOption(value = {"-p","--port"}, defaultValue = "6379") String port,
            @ShellOption(value = {"-s","--application-name"}, defaultValue = "smartcache") String applicationName,
            @ShellOption(value = {"-t","--ttl"}, help = "The time to live for all stored query results. Must be in the format of a duration (e.g., 30m, 2h)") String ttlStr,
            @ShellOption(value = {"-m","--match"}, defaultValue = "", help = "The value(s) for the rule to match against. Required if type is not 'any'") String match,
            @ShellOption(value = {"-k","--type"}, help = "The Rule Type (required). Valid values are 'any', 'tables-any', 'tables-all', 'tables-exact', 'query-ids', and 'regex'.") String type)
    {
        try{
            Duration ttl = Duration.valueOf(ttlStr);
            if(!Objects.equals(type.toLowerCase(), "any") && match.isEmpty()){
                throw new Exception("Please provide a value for the 'match' argument. This is required when the Rule Type is anything other than 'any'.");
            }
            List<String> matchSplit = Arrays.stream(match.split(",")).toList();

            RuleConfig rule = new RuleConfig();
            rule.setTtl(ttl);
            switch(type.toLowerCase()){
                case "any":
                    break;
                case "tables-all":
                    rule.setTablesAll(matchSplit);
                    break;
                case "tables-any":
                    rule.setTablesAny(matchSplit);
                    break;
                case "tables-exact":
                    rule.setTables(matchSplit);
                    break;
                case "query-ids":
                    rule.setQueryIds(matchSplit);
                    break;
                case "regex":
                    rule.setRegex(match);
                    break;
                default:
                    throw new Exception("Invalid rule type '" + type.toLowerCase() + "'. Valid types are 'any', 'tables-any', 'tables-all', 'tables-exact', 'query-ids', and 'regex'.");
            }

            RedisService client = initializeClient(host, port, applicationName);
            List<RuleConfig> rules = new ArrayList<>(client.getRules());
            rules.add(0,rule);
            client.commitRules(rules);
        } catch (Exception ex){
            System.out.printf("Error when making rule: %s%nExiting. . .%n", ex);
            System.exit(1);
        }

        System.out.println("Rule created.");
        System.exit(0);
        return "Rule Created";
    }

    @ShellMethod(key="Interactive")
    public String interactive(
            @ShellOption(value = {"-n","--hostname"}, defaultValue = "localhost")String host,
            @ShellOption(value = {"-p","--port"}, defaultValue = "6379") String port,
            @ShellOption(value = {"-s","--application-name"}, defaultValue = "smartcache") String applicationName
    ){
        try{
            RedisService client = initializeClient(host, port, applicationName);

            String[] options = {LIST_APPLICATION_QUERIES, LIST_TABLES, CREATE_RULE, LIST_RULES, CLEAR_METRICS, RESET_CONFIG, EXIT};

            String nextAction = "";
            TableSelector.SingleItemSelectorContext<Action, SelectorItem<Action>> context = TableSelector.SingleItemSelectorContext.empty(1, "");

            while(!nextAction.equals(EXIT)){

                List<SelectorItem<Action>> actions = Arrays.stream(options).map(x->SelectorItem.of(x,new Action(x))).collect(Collectors.toList());
                TableSelector<Action, SelectorItem<Action>> component = new TableSelector<>(getTerminal(),
                        actions, "Select action", null, "Select Action", true, 1, "");
                component.setResourceLoader(getResourceLoader());
                component.setTemplateExecutor(getTemplateExecutor());

                context = component.run(context);

                if(component.isEscapeMode()){
                    System.exit(0);
                }

                if(context.getResultItem().isPresent()){
                    nextAction = context.getResultItem().get().getItem().getAction();

                    switch (nextAction) {
                        case CREATE_RULE -> createRule(client);
                        case LIST_APPLICATION_QUERIES -> queryTable(client);
                        case LIST_TABLES -> tablesTable(client);
                        case LIST_RULES -> ruleTable(client);
                        case CLEAR_METRICS -> clearMetrics(client);
                        case RESET_CONFIG -> resetConfig(client);
                    }
                }

                getTerminal().puts(InfoCmp.Capability.clear_screen);
            }

            System.exit(0);
        }
        catch (Exception ex){
            if(ex.getMessage() == null){
                System.out.printf("Encountered fatal error: %s%nExiting. . .%n", ex);
                throw ex;

            } else{
                System.out.printf("Encountered fatal error: %s%nExiting. . .%n", ex.getMessage());
            }

            System.exit(1);
        }


        return "Interactive!";
    }

    public boolean confirm(String message){
        ConfirmationInputExtension component = new ConfirmationInputExtension(getTerminal(), message, false);
        component.setResourceLoader(getResourceLoader());
        component.setTemplateExecutor(getTemplateExecutor());
        ConfirmationInput.ConfirmationInputContext context = component.run(ConfirmationInput.ConfirmationInputContext.empty());
        return !component.isEscapeMode() && context.getResultValue();
    }

    public void clearMetrics(RedisService client){
        String prompt = "Are you sure you want to reset Redis Smart Cache's metrics?";
        if(confirm(prompt)){
            client.clearMetrics();
        }
    }

    public void resetConfig(RedisService client){
        String prompt = "Are you sure you want to disable all caching from Smart Cache?";
        if(confirm(prompt)){
            client.commitRules(new ArrayList<>());
        }
    }

    public void ruleTable(RedisService client){
        List<RuleInfo> rules = client.getRules().stream().map(x->new RuleInfo(x, RuleInfo.Status.Current)).collect(Collectors.toList());
        String instructions = "Press [ENTER] to edit an existing rule\nPress 'n' to create a new rule\nPress 'd' to delete a rule\nPress 'c' to commit\nPress [ESC] to go back\nPress [CTRL+C] to exit\n\n";
        int cursorRow = 0;

        TableSelector.SingleItemSelectorContext<RuleInfo, SelectorItem<RuleInfo>> context = TableSelector.SingleItemSelectorContext.empty(3, instructions);

        while (true){
            getTerminal().puts(InfoCmp.Capability.clear_screen);

            List<SelectorItem<RuleInfo>> ruleInfos = rules.stream().map(rule -> SelectorItem.of(UUID.randomUUID().toString(),rule)).collect(Collectors.toList());

            TableSelector<RuleInfo, SelectorItem<RuleInfo>> component = new TableSelector<>(getTerminal(),
                    ruleInfos, "rules", null, RuleInfo.getHeaderRow((getTerminal().getWidth() - 10) / 4), true, 4, instructions);
            component.setResourceLoader(getResourceLoader());
            component.setTemplateExecutor(getTemplateExecutor());
            context.setCursorRow(cursorRow);
            context = component.run(context);
            cursorRow = context.getCursorRow();
            Optional<SelectorItem<RuleInfo>> res = context.getResultItem();

            if(component.isConfirmMode()){
                ComponentFlow flow = componentFlowBuilder.clone().reset()
                        .withConfirmationInput("Confirm")
                        .name("Would you like to commit this new configuration?")
                        .next(null)
                        .template("classpath:confirmation-input.stg")
                        .and()
                    .build();
                boolean confirmed = flow.run().getContext().get("Confirm");
                if(confirmed){
                    List<RuleConfig> rulesToCommit = rules.stream().filter(rule->rule.getStatus() != RuleInfo.Status.Delete).map(RuleInfo::getRule).collect(Collectors.toList());
                    client.commitRules(rulesToCommit);
                    try{
                        Thread.sleep(250);
                    } catch (Exception e){
                        //ignored
                    }

                    rules = client.getRules().stream().map(x->new RuleInfo(x, RuleInfo.Status.Current)).collect(Collectors.toList());
                }

                component.setConfirmMode(false);
                continue;
            }
            if(!component.isEscapeMode() && res.isPresent()){
                Optional<RuleConfig> updatedRule = newRuleDialogCustom(false);
                updatedRule.ifPresent(ruleConfig ->{
                    res.get().getItem().setRule(ruleConfig);
                    res.get().getItem().setStatus(RuleInfo.Status.Editing);
                });
            }
            else if(component.isNewMode()){
                Optional<RuleConfig> newRule = newRuleDialogCustom(false);
                if (newRule.isPresent()){
                    rules.add(0, new RuleInfo(newRule.get(), RuleInfo.Status.New));
                }
                component.setNewMode(false);
            }
            else if(component.isDeleteMode()){
                int rowNum = context.getCursorRow();
                RuleInfo rule = context.getItems().get(rowNum).getItem();
                if(rule.getStatus() == RuleInfo.Status.New){
                    rules.remove(rowNum);
                }
                else {
                    rules.get(rowNum).setStatus(RuleInfo.Status.Delete);
                }

                component.setDeleteMode(false);
            }
            else{
                break;
            }

        }
    }

    public void tablesTable(RedisService client){
        String instructions = "Press [ENTER] to edit\nPress [ESC] to go back\nPress ctrl+c to exit\n\n";
        int cursorRow = 0;
        TableSelector.SingleItemSelectorContext<TableInfo, SelectorItem<TableInfo>> context = TableSelector.SingleItemSelectorContext.empty(4, instructions);
        while(true){
            getTerminal().puts(InfoCmp.Capability.clear_screen);
            List<SelectorItem<TableInfo>> tables = new ArrayList<>();
            for(TableInfo tableInfo : client.getTables()){
                tables.add(SelectorItem.of(tableInfo.getName(), tableInfo));
            }

            TableSelector<TableInfo, SelectorItem<TableInfo>> component = new TableSelector<>(getTerminal(),
                    tables, "tables", null, TableInfo.headerRow((getTerminal().getWidth() - 10) / 4), true, 4, instructions);
            component.setResourceLoader(getResourceLoader());
            component.setTemplateExecutor(getTemplateExecutor());
            context.setCursorRow(cursorRow);
            context = component.run(context);
            cursorRow = context.getCursorRow();
            Optional<SelectorItem<TableInfo>> res = context.getResultItem();

            if(component.isConfirmMode()){
                component.setConfirmMode(false);
                continue;
            }
            if(!component.isEscapeMode() && res.isPresent()){
                Optional<Duration> duration = getTtl(String.format("Create rule to cache table: %s", res.get().getName()));
                duration.ifPresent(ttl->{
                    RuleConfig newRule = new RuleConfig();
                    newRule.setTtl(ttl);
                    newRule.setTablesAny(Arrays.asList(res.get().getName().split(",")));
                    Optional<Boolean> confirmed = getConfirmation(new RuleInfo(newRule, RuleInfo.Status.New));
                    confirmed.ifPresent(c->{
                        if(c){
                            List<RuleConfig> rules = new ArrayList<>(client.getRules());
                            rules.add(0, newRule);
                            client.commitRules(rules);
                        }
                    });
                });
            }
            else{
                break;
            }
        }
    }

    private List<SelectorItem<QueryInfo>> getQueries(RedisService client){
        List<SelectorItem<QueryInfo>> queries = new ArrayList<>();

        for (QueryInfo q : client.getQueries()){
            queries.add(SelectorItem.of(q.getQueryId(),q));
        }
        return queries;
    }

    public void queryTable(RedisService client){
        List<RuleConfig> rules = new ArrayList<>(client.getRules());

        Map<Duration, RuleConfig> pendingRules = new HashMap<>();

        List<SelectorItem<QueryInfo>> queries = getQueries(client);
        int cursorRow = 0;
        TableSelector.SingleItemSelectorContext<QueryInfo, SelectorItem<QueryInfo>> context = TableSelector.SingleItemSelectorContext.empty(8, tableInstructions);

        while(true){
            getTerminal().puts(InfoCmp.Capability.clear_screen);
            TableSelector<QueryInfo, SelectorItem<QueryInfo>> component = new TableSelector<>(getTerminal(),
                    queries, "queries", null, QueryInfo.getHeaderRow((getTerminal().getWidth()-10)/8, true), true, 8, tableInstructions);
            component.setResourceLoader(getResourceLoader());
            component.setTemplateExecutor(getTemplateExecutor());

            context.setCursorRow(cursorRow);
            context = component.run(context);
            cursorRow = context.getCursorRow();
            Optional<SelectorItem<QueryInfo>> resOpt = context.getResultItem();

            if (component.isConfirmMode()){
                Set<String> validResponses = new HashSet<>(Arrays.asList("y","Y","n","N"));
                Optional<Boolean> confirmed = Optional.empty();
                while(!confirmed.isPresent()){

                    String prompt = "Confirm pending updates (y/N)";
                    StringInput stringInputComponent = new StringInput(getTerminal(), prompt,"n");
                    stringInputComponent.setResourceLoader(getResourceLoader());
                    stringInputComponent.setTemplateExecutor(getTemplateExecutor());
                    StringInput.StringInputContext stringInputContext = stringInputComponent.run(StringInput.StringInputContext.empty());
                    String confirmationInput = stringInputContext.getResultValue();
                    if(validResponses.contains(confirmationInput)){
                        confirmed = Optional.of(confirmationInput.equalsIgnoreCase("y"));
                    }
                    else{
                        continue;
                    }

                    if(confirmed.get()){
                        for(RuleConfig rule : pendingRules.values()){
                            rules.add(0, rule);
                        }

                        client.commitRules(rules);
                        queries = getQueries(client);
                    }
                    else{
                        component.setConfirmMode(false);
                    }
                }
            }
            else if (!component.isEscapeMode() && resOpt.isPresent()){

                QueryInfo result = resOpt.get().getItem();

                String info = result.toFormattedString(getTerminal().getWidth());
                Optional<Duration> duration = getTtl(info);

                if(!duration.isPresent()){
                    continue;
                }

                RuleConfig rule;
                if(pendingRules.containsKey(duration.get())){
                    pendingRules.get(duration.get()).getQueryIds().add(result.getQueryId());
                    rule = pendingRules.get(duration.get());
                }else{
                    rule = new RuleConfig();
                    rule.setQueryIds(List.of(result.getQueryId()));
                    rule.setTtl(duration.get());
                    pendingRules.put(duration.get(),rule);
                }
                queries.get(context.getCursorRow()).getItem().setPendingRule(rule);
            }
            else{
                break;
            }
        }

    }
}
