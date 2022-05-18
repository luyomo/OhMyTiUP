#!/bin/bash

export PATH=/opt/tidb-v6.0.0-linux-amd64/bin:/usr/local/bin:/usr/bin:/bin:/usr/games:/bin:/sbin:/usr/bin:/usr/sbin
export LD_LIBRARY_PATH=/opt/oracle/instantclient_21_4:
export PATH=$LD_LIBRARY_PATH:$PATH

sqlplus {{ .DBUser }}/{{ .DBPassword }}@{{ .DBHost }}:{{ .DBPort }}/{{ .DBName }} << EOF
DROP TABLE test.test01;
DROP USER test;
create user test identified by test account unlock default tablespace users;
grant resource,connect to test;
alter user test quota unlimited on users;
create table test.test01(col01 int primary key, col02 int);
create or replace procedure do_truncate(table_name     in varchar2,
                                          partition_name in varchar2) as
begin
  if partition_name || 'x' = 'x' then
    execute immediate 'truncate table ' || table_name;
  else
    execute immediate 'alter table ' || table_name || ' truncate partition ' || partition_name;
  end if;
end;
/
EOF
