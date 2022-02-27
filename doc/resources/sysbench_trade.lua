-- Called by sysbench one time to initialize this script
function thread_init()

  -- Create globals to be used elsewhere in the script

  -- drv - initialize the sysbench mysql driver
  drv = sysbench.sql.driver()

  -- con - represents the connection to MySQL
  con = drv:connect()
end

-- Called by sysbench when script is done executing
function thread_done()
  -- Disconnect/close connection to MySQL
  con:disconnect()
end

-- Called by sysbench for each execution
function event()

  -- If user requested to disable transactions,
  -- do not execute BEGIN statement
  if not sysbench.opt.skip_trx then
    con:query("BEGIN")
  end

  -- Custom statements
  -- Order generate
  execute_insert_order()

  -- Trade Generate
  execute_insert_trade()

  -- Order select
  execute_select_order()

  -- Trade select
  execute_select_trade()

  -- Like above, if transactions are disabled,
  -- do not execute COMMIT
  if not sysbench.opt.skip_trx then
    con:query("COMMIT")
  end
end

if sysbench.cmdline.command == nil then
   error("Command is required. Supported commands: run")
end

sysbench.cmdline.options = {
  point_selects = {"Number of point SELECT queries to run", 5},
  skip_trx = {"Do not use BEGIN/COMMIT; Use global auto_commit value", false}
}

function execute_select_order()

  rs = con:query("select max(order_id) as max_id from order_table")
  for i = 1, rs.nrows do
    maxId = unpack(rs:fetch_row(), 1, rs.nfields)
  end

--  print(string.format("The max id is <%d>", maxId))
  rs = con:query(string.format([[
  select order_id
           , t3.name as security_name
           , t2.name as client_name
           , t1.price
           , t1.quantity
           , case when t1.buy_sell_flag = 0 then 'buy' else 'sell' end
           from order_table t1
     inner join client_table t2
             on t1.client_id = t2.id
            and t1.order_id = %d
     inner join security_table t3
             on t1.security_id = t3.id
      ]], math.random(maxId) ))

end

function execute_select_trade()

  rs = con:query("select max(order_id) as max_id from trade_table")
  for i = 1, rs.nrows do
    maxId = unpack(rs:fetch_row(), 1, rs.nfields)
  end
  if maxId == nil then
     return
  end 

--  print(string.format("The max id is <%d>", maxId))
  rs = con:query(string.format([[
    select t4.order_id
         , t1.trade_id
         , t3.name as security_name
         , t2.name as client_name
         , t1.price
         , t1.quantity
         , case when t1.buy_sell_flag = 0 then 'buy' else 'sell' end as buy_sell
      from trade_table t1
inner join client_table t2
        on t1.client_id = t2.id
       and trade_id = %d
inner join security_table t3
        on t1.security_id = t3.id
inner join order_table t4
        on t1.order_id = t4.order_id
      ]], math.random(maxId) ))
end

function execute_insert_order()
  security_id = sb_rand(1, 100)
  client_id = sb_rand(1, 100)
  price = sb_rand(10000,  100000)
  buy_sell_flag = sb_rand(0, 1)
  quantity = sb_rand(1000, 10000)

  con:query(string.format([[
    INSERT INTO order_table(security_id, client_id, price, buy_sell_flag, quantity) values(%d, %d, %d, %d, %d)
      ]], security_id, client_id, price, buy_sell_flag, quantity))
end

function execute_insert_trade()

  -- Generate trade one time per 100
  trade_flag = math.random(100)
  if trade_flag ~= 50 then
    return
  end

  -- Get max order id
  rs = con:query("select max(order_id) as max_id from order_table")
  for i = 1, rs.nrows do
    maxId = unpack(rs:fetch_row(), 1, rs.nfields)
  end

  -- Generate one order to be proceed
  order_id = math.random(maxId)

  -- Select info from order table
  rs = con:query(string.format([[
    select security_id, price, buy_sell_flag, client_id, quantity from order_table where order_id = %d
      ]], order_id))

  -- If record exists, fetch all the data
  if rs.nrows > 0 then
    for i = 1, rs.nrows do
      row = rs:fetch_row()
      security_id = unpack(row, 1, rs.nfields)
      price = unpack(row, 2, rs.nfields)
      buy_sell_flag = unpack(row, 3, rs.nfields)
      client_id = unpack(row, 4, rs.nfields)
      total_quantity = unpack(row, 5, rs.nfields)
    end
  end

  -- Get the trade quantity
  rs = con:query(string.format([[
    select sum(quantity) as quantity from trade_table where order_id = %d
      ]], order_id))
  if rs.nrows > 0 then
    for i = 1, rs.nrows do
      traded_quantity = unpack(rs:fetch_row(), 1, rs.nfields)
    end
  end

  if traded_quantity == nil then
    traded_quantity = 0
  end

  -- If the proceeded quantity is more than total quantity
  if tonumber(traded_quantity) >=  tonumber(total_quantity) then
    return
  end

  -- Calculate the quantity from remaining quantity
  trade_quantity = math.random(total_quantity - traded_quantity)
  --print(string.format("security is: <%d>, The total quantity is <%d> and trade quantity is <%d>, quantity to trade <%d>", security_id, total_quantity, traded_quantity, trade_quantity))

  con:query(string.format([[
    INSERT INTO trade_table(order_id, security_id, client_id, price, buy_sell_flag, quantity) values(%d, %d, %d, %d, %d, %d)
      ]], order_id, security_id, client_id, price, buy_sell_flag, trade_quantity))
end

function prepare()
  local drv = sysbench.sql.driver()
  local con = drv:connect()

  print("Creating table order_table")
  con:query(string.format([[
    CREATE TABLE if not exists order_table (
            order_id int primary key auto_increment,
            security_id int not null,
            price decimal(20, 8) not null,
            buy_sell_flag boolean not null,
            quantity bigint not null,
            client_id int not null,
            create_timestamp timestamp default current_timestamp,
            create_user varchar(128) ,
            update_timestamp timestamp default current_timestamp,
            update_user varchar(128) 
            )
      ]]))

  print("Creating table trade_table")
  con:query(string.format([[
     CREATE TABLE if not exists trade_table (
            trade_id int primary key auto_increment,
            order_id int not null,
            security_id int not null,
            price decimal(20, 8) not null,
            buy_sell_flag boolean not null,
            quantity bigint not null,
            client_id int not null,
            create_timestamp timestamp default current_timestamp,
            create_user varchar(128),
            update_timestamp timestamp default current_timestamp,
            update_user varchar(128)
            )
      ]]))

  print("Creating table client_table")
  con:query(string.format([[
       CREATE TABLE if not exists client_table(
       id int primary key auto_increment,
       name varchar(128) not null,
       margin_type boolean default false
       )
      ]]))

  print("Creating table security_table")
  con:query(string.format([[
       CREATE TABLE if not exists security_table(
       id int primary key auto_increment,
       name varchar(128) not null,
       margin_type boolean default false
       )
      ]]))

  local pad_value_template = "###########-###########-###########"


  -- Generate 100 rows client data for testing
  for i=1, 100 do
  con:query(string.format([[
    insert into client_table(name) values('%s')
  -- string.rep("@", sysbench.rand.special(2, 15))
  ]], sysbench.rand.string(string.rep("@", sysbench.rand.special(2, 15)))))
  end

  -- Generate 100 rows security data for testing
  for i=1, 100 do
  con:query(string.format([[
    insert into security_table(name) values('%s')
  -- string.rep("@", sysbench.rand.special(2, 15))
  ]], sysbench.rand.string(string.rep("@", sysbench.rand.special(2, 15)))))
  end

end

function cleanup()
   local drv = sysbench.sql.driver()
   local con = drv:connect()

  print("Cleaning the table order_table ")
  con:query(string.format([[
      DROP TABLE IF EXISTS order_table
      ]]))

  print("Cleaning the table trade_table ")
  con:query(string.format([[
      DROP TABLE IF EXISTS trade_table
      ]]))

  print("Cleaning the table client_table ")
  con:query(string.format([[
      DROP TABLE IF EXISTS client_table
      ]]))

  print("Cleaning the table security_table ")
  con:query(string.format([[
      DROP TABLE IF EXISTS security_table
      ]]))
end
