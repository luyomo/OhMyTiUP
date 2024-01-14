#!/usr/bin/sysbench
-- -------------------------------------------------------------------------- --
-- Bulk insert benchmark: do multi-row INSERTs concurrently in --threads
-- threads with each thread inserting into its own table. The number of INSERTs
-- executed by each thread is controlled by either --time or --events.
-- -------------------------------------------------------------------------- --

require("tidb_common")

cursize=0

function thread_init()
   drv = sysbench.sql.driver()
   con = drv:connect()
end

function prepare()
   local i

   local drv = sysbench.sql.driver()
   local con = drv:connect()

   for i = 1, sysbench.opt.threads do
      print("Creating table 'sbtest" .. i .. "'...")
      con:query(string.format([[
        CREATE TABLE IF NOT EXISTS sbtest%d (
          id BIGINT AUTO_RANDOM PRIMARY KEY CLUSTERED,
          k LONGTEXT NOT NULL) ]], i))
   end
end

function event()
    local s = sysbench.rand.string(string.rep('@', sysbench.rand.default(sysbench.opt.rand_string_len, sysbench.opt.rand_string_len)))
    if (cursize == 0) then
       con:bulk_insert_init("INSERT INTO sbtest" .. thread_id+1 .. "(k) VALUES")
    end
    if (cursize%sysbench.opt.bulk_inserts == 0) then
       con:bulk_insert_done()
       con:bulk_insert_init("INSERT INTO sbtest" .. thread_id+1 .. "(k) VALUES")
    end

    cursize = cursize + 1
    con:bulk_insert_next("('" .. s .. "')")
end

function thread_done(thread_9d)
   con:bulk_insert_done()
   con:disconnect()
end

function cleanup()
   local i

   local drv = sysbench.sql.driver()
   local con = drv:connect()

   for i = 1, sysbench.opt.threads do
      print("Dropping table 'sbtest" .. i .. "'...")
      con:query("DROP TABLE IF EXISTS sbtest" .. i )
   end
end

sysbench.hooks.report_intermediate = report_noop
sysbench.hooks.report_cumulative = report_simple_cumulative
