#!/bin/bash

database=$1
source_table_name=$2
target_table_name=$3
op_type=$4


COLUMNS="Year, Quarter, Month, DayofMonth, DayOfWeek, FlightDate, Reporting_Airline, DOT_ID_Reporting_Airline, IATA_CODE_Reporting_Airline, Tail_Number, Flight_Number_Reporting_Airline, OriginAirportID, OriginAirportSeqID, OriginCityMarketID, Origin, OriginCityName, OriginState, OriginStateFips, OriginStateName, OriginWac, DestAirportID, DestAirportSeqID, DestCityMarketID, Dest, DestCityName, DestState, DestStateFips, DestStateName, DestWac, CRSDepTime, DepTime, DepDelay, DepDelayMinutes, DepDel15, DepartureDelayGroups, DepTimeBlk, TaxiOut, WheelsOff, WheelsOn, TaxiIn, CRSArrTime, ArrTime, ArrDelay, ArrDelayMinutes, ArrDel15, ArrivalDelayGroups, ArrTimeBlk, Cancelled, CancellationCode, Diverted, CRSElapsedTime, ActualElapsedTime, AirTime, Flights, Distance, DistanceGroup, CarrierDelay, WeatherDelay, NASDelay, SecurityDelay, LateAircraftDelay, FirstDepTime, TotalAddGTime, LongestAddGTime, DivAirportLandings, DivReachedDest, DivActualElapsedTime, DivArrDelay, DivDistance, Div1Airport, Div1AirportID, Div1AirportSeqID, Div1WheelsOn, Div1TotalGTime, Div1LongestGTime, Div1WheelsOff, Div1TailNum, Div2Airport, Div2AirportID, Div2AirportSeqID, Div2WheelsOn, Div2TotalGTime, Div2LongestGTime, Div2WheelsOff, Div2TailNum, Div3Airport, Div3AirportID, Div3AirportSeqID, Div3WheelsOn, Div3TotalGTime, Div3LongestGTime, Div3WheelsOff, Div3TailNum, Div4Airport, Div4AirportID, Div4AirportSeqID, Div4WheelsOn, Div4TotalGTime, Div4LongestGTime, Div4WheelsOff, Div4TailNum, Div5Airport, Div5AirportID, Div5AirportSeqID, Div5WheelsOn, Div5TotalGTime, Div5LongestGTime, Div5WheelsOff, Div5TailNum, timestamp_tidb"

if [ "$op_type" == "partition" ];
then
/opt/scripts/run_tidb_query $database batchusr 1234Abcd "batch on $source_table_name.id limit 2000 insert into $target_table_name( $COLUMNS) select $COLUMNS from $source_table_name where DayOfWeek = 1 "
else
/opt/scripts/run_tidb_query $database batchusr 1234Abcd "insert into $target_table_name( $COLUMNS) select $COLUMNS from $source_table_name where DayOfWeek = 1 "
fi
