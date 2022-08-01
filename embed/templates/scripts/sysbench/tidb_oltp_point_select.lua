require("oltp_common")
require("tidb_common")

function prepare_statements()
   -- use 1 query per event, rather than sysbench.opt.point_selects which
   -- defaults to 10 in other OLTP scripts
   sysbench.opt.point_selects=1

   prepare_point_selects()
end

function event()
   execute_point_selects()
end

sysbench.hooks.report_intermediate = report_noop
sysbench.hooks.report_cumulative = report_cumulative
