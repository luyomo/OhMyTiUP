#!/bin/bash

usage="$(basename "$0") start_year start_month end_year end_month -- Import the data from ontime between start_year/start_month and end_year/end_month"
if [ $# -ne 4  ]; then
    echo usage
    exit 1
fi
start_year=$1
start_month=$2
end_year=$3
end_year=$4

for s in `seq 2018 2018`
do
for m in `seq 1 12`
do
wget https://transtats.bts.gov/PREZIP/On_Time_Reporting_Carrier_On_Time_Performance_1987_present_${s}_${m}.zip --no-check-certificate
unzip On_Time_Reporting_Carrier_On_Time_Performance_1987_present_${s}_${m}.zip
mv "On_Time_Reporting_Carrier_On_Time_Performance_(1987_present)_${s}_${m}.csv" "${s}_${m}.csv";
rm -f readme.html

mysql -h ae6c7ecfe42b34d7b9414bbff4db3f50-6cdc6893e8dd5a0a.elb.ap-northeast-1.amazonaws.com -P 4000 -u root -p1234Abcd test -e "LOAD DATA LOCAL INFILE '$(pwd)/${s}_${m}.csv' INTO TABLE ontime FIELDS TERMINATED BY ',' ENCLOSED BY '\"' LINES TERMINATED BY '\n' (Year, Quarter, Month, DayofMonth, DayOfWeek, FlightDate, UniqueCarrier, AirlineID, Carrier, TailNum, FlightNum, OriginAirportID, OriginAirportSeqID, OriginCityMarketID, Origin, OriginCityName, OriginState, OriginStateFips, OriginStateName, OriginWac, DestAirportID, DestAirportSeqID, DestCityMarketID, Dest, DestCityName, DestState, DestStateFips, DestStateName, DestWac, CRSDepTime, DepTime, DepDelay, DepDelayMinutes, DepDel15, DepartureDelayGroups, DepTimeBlk, TaxiOut, WheelsOff, WheelsOn, TaxiIn, CRSArrTime, ArrTime, ArrDelay, ArrDelayMinutes, ArrDel15, ArrivalDelayGroups, ArrTimeBlk, Cancelled, CancellationCode, Diverted, CRSElapsedTime, ActualElapsedTime, AirTime, Flights, Distance, DistanceGroup, CarrierDelay, WeatherDelay, NASDelay, SecurityDelay, LateAircraftDelay, FirstDepTime, TotalAddGTime, LongestAddGTime, DivAirportLandings, DivReachedDest, DivActualElapsedTime, DivArrDelay, DivDistance, Div1Airport, Div1AirportID, Div1AirportSeqID, Div1WheelsOn, Div1TotalGTime, Div1LongestGTime, Div1WheelsOff, Div1TailNum, Div2Airport, Div2AirportID, Div2AirportSeqID, Div2WheelsOn, Div2TotalGTime, Div2LongestGTime, Div2WheelsOff, Div2TailNum, Div3Airport, Div3AirportID, Div3AirportSeqID, Div3WheelsOn, Div3TotalGTime, Div3LongestGTime, Div3WheelsOff, Div3TailNum, Div4Airport, Div4AirportID, Div4AirportSeqID, Div4WheelsOn, Div4TotalGTime, Div4LongestGTime, Div4WheelsOff, Div3TailNum, Div5Airport, Div5AirportID, Div5AirportSeqID, Div5WheelsOn, Div5TotalGTime, Div5LongestGTime, Div5WheelsOff, Div5TailNum )"
rm On_Time_Reporting_Carrier_On_Time_Performance_1987_present_${s}_${m}.zip
rm "${s}_${m}.csv"
done
done
