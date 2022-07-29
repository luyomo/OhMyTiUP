#!/bin/bash

usage="$(basename "$0") database table_name start_year start_month end_year end_month -- Import the data from ontime between start_year/start_month and end_year/end_month"
if [ $# -ne 6  ]; then
    echo $usage
    exit 1
fi
database=$1
table_name=$2
start_year=$3
start_month=$4
end_year=$5
end_month=$6

for s in `seq $start_year $end_year`
do
for m in `seq $start_month $end_month`
do
wget https://transtats.bts.gov/PREZIP/On_Time_Reporting_Carrier_On_Time_Performance_1987_present_${s}_${m}.zip --no-check-certificate
unzip On_Time_Reporting_Carrier_On_Time_Performance_1987_present_${s}_${m}.zip
mv "On_Time_Reporting_Carrier_On_Time_Performance_(1987_present)_${s}_${m}.csv" "${s}_${m}.csv";
rm -f readme.html

/opt/scripts/run_tidb_query $database "LOAD DATA LOCAL INFILE '$(pwd)/${s}_${m}.csv' INTO TABLE ${table_name} FIELDS TERMINATED BY ',' ENCLOSED BY '\"' LINES TERMINATED BY '\n' ignore 1 lines (Year, Quarter, Month, DayofMonth, DayOfWeek, FlightDate, UniqueCarrier, AirlineID, Carrier, TailNum, FlightNum, OriginAirportID, OriginAirportSeqID, OriginCityMarketID, Origin, OriginCityName, OriginState, OriginStateFips, OriginStateName, OriginWac, DestAirportID, DestAirportSeqID, DestCityMarketID, Dest, DestCityName, DestState, DestStateFips, DestStateName, DestWac, CRSDepTime, DepTime, DepDelay, DepDelayMinutes, DepDel15, DepartureDelayGroups, DepTimeBlk, TaxiOut, WheelsOff, WheelsOn, TaxiIn, CRSArrTime, ArrTime, ArrDelay, ArrDelayMinutes, ArrDel15, ArrivalDelayGroups, ArrTimeBlk, Cancelled, CancellationCode, Diverted, CRSElapsedTime, ActualElapsedTime, AirTime, Flights, Distance, DistanceGroup, CarrierDelay, WeatherDelay, NASDelay, SecurityDelay, LateAircraftDelay, FirstDepTime, TotalAddGTime, LongestAddGTime, DivAirportLandings, DivReachedDest, DivActualElapsedTime, DivArrDelay, DivDistance, Div1Airport, Div1AirportID, Div1AirportSeqID, Div1WheelsOn, Div1TotalGTime, Div1LongestGTime, Div1WheelsOff, Div1TailNum, Div2Airport, Div2AirportID, Div2AirportSeqID, Div2WheelsOn, Div2TotalGTime, Div2LongestGTime, Div2WheelsOff, Div2TailNum, Div3Airport, Div3AirportID, Div3AirportSeqID, Div3WheelsOn, Div3TotalGTime, Div3LongestGTime, Div3WheelsOff, Div3TailNum, Div4Airport, Div4AirportID, Div4AirportSeqID, Div4WheelsOn, Div4TotalGTime, Div4LongestGTime, Div4WheelsOff, Div4TailNum, Div5Airport, Div5AirportID, Div5AirportSeqID, Div5WheelsOn, Div5TotalGTime, Div5LongestGTime, Div5WheelsOff, Div5TailNum )"
rm On_Time_Reporting_Carrier_On_Time_Performance_1987_present_${s}_${m}.zip
rm "${s}_${m}.csv"
done
done
