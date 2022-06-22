#!/bin/bash

mysqlslap --no-defaults -h linetest-ee7c826a317c73cf.elb.us-east-1.amazonaws.com -P 4000 --user=root --password=1234Abcd --delimiter=";" --create-schema="test02" --create=table.sql --query=test.sql --concurrency=4000 --iterations=1  --number-of-queries=300000000 --no-drop
