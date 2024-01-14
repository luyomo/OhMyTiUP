sysbench.cmdline.options = {
       bulk_inserts = {"Number of values to to insert", 1000},
          rand_string_len = {"rand string length to insert", 1024}
          
}

function report_noop(stat)
end

function report_cumulative(stat)
print("---------- Result summary ----------")
print(string.format("%s,%s,%s,%s," ..
                        "%s,%s," ..
                        "%s,%s," ..
                        "%s,%s," ..
                        "%s,%s," ..
                        "%s,%s,%s,%2dth_%s ms,%s,",
                    "reads", "writes", "other", "queries",
                    "events", "events/sec",
                    "queries", "queries/sec",
                    "errors", "errors/sec",
                    "reconnects", "reconnects/sec",
                    "latency_min(ms)", "latency_avg(ms)", "latency_max(ms)", sysbench.opt.percentile , "latency_pct (ms)", "latency_sum (ms)"
    ))

    local queries = stat.reads + stat.writes + stat.other
    local seconds = stat.time_total
    print(string.format("%u,%u,%u,%u," ..
                        "%u,%.2f," ..
                        "%u,%.2f," ..
                        "%u,%.2f," ..
                        "%u,%.2f," ..
                        "%.2f,%.2f,%.2f,%.2f,%.2f",
                    stat.reads, stat.writes, stat.other, queries,
                    stat.events, stat.events / seconds,
                    queries, queries / seconds,
                    stat.errors, stat.errors / seconds,
                    stat.reconnects, stat.reconnects / seconds,
                    stat.latency_min*1000, stat.latency_avg*1000, stat.latency_max*1000, stat.latency_pct*1000, stat.latency_sum*1000
    ))
    print("---------- End result summary ----------")
end

function report_simple_cumulative(stat)
print("---------- Result summary ----------")
print(string.format("%s,%s,%s," ..
                        "%s,%s," ..
                        "%s,%s," ..
                        "%s,%s,%s,%2dth_%s ms,%s,",
                    "reads", "writes", "queries",
                    "events", "events/sec",
                    "queries", "queries/sec",
                    "latency_min(ms)", "latency_avg(ms)", "latency_max(ms)", sysbench.opt.percentile , "latency_pct (ms)", "latency_sum (ms)"
    ))

    local queries = stat.reads + stat.writes + stat.other
    local seconds = stat.time_total
    print(string.format("%u,%u,%u," ..
                        "%u,%.2f," ..
                        "%u,%.2f," ..
                        "%.2f,%.2f,%.2f,%.2f,%.2f",
                    stat.reads, stat.writes, queries,
                    stat.events, stat.events / seconds,
                    queries, queries / seconds,
                    stat.latency_min*1000, stat.latency_avg*1000, stat.latency_max*1000, stat.latency_pct*1000, stat.latency_sum*1000
    ))
    print("---------- End result summary ----------")
end
