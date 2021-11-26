CREATE TABLE `ontime` (
   id bigint primary key auto_increment, 
  `Year` year(4) DEFAULT NULL,
  `Quarter` tinyint(4) DEFAULT NULL,
  `Month` tinyint(4) DEFAULT NULL,
  `DayofMonth` tinyint(4) DEFAULT NULL,
  `DayOfWeek` tinyint(4) DEFAULT NULL,
  `FlightDate` date DEFAULT NULL,
  `UniqueCarrier` char(7) DEFAULT NULL,
  `AirlineID` int(11) DEFAULT NULL,
  `Carrier` char(2) DEFAULT NULL,
  `TailNum` varchar(50) DEFAULT NULL,
  `FlightNum` varchar(10) DEFAULT NULL,
  `OriginAirportID` int(11) DEFAULT NULL,
  `OriginAirportSeqID` int(11) DEFAULT NULL,
  `OriginCityMarketID` int(11) DEFAULT NULL,
  `Origin` char(5) DEFAULT NULL,
  `OriginCityName` varchar(100) DEFAULT NULL,
  `OriginState` char(2) DEFAULT NULL,
  `OriginStateFips` varchar(10) DEFAULT NULL,
  `OriginStateName` varchar(100) DEFAULT NULL,
  `OriginWac` int(11) DEFAULT NULL,
  `DestAirportID` int(11) DEFAULT NULL,
  `DestAirportSeqID` int(11) DEFAULT NULL,
  `DestCityMarketID` int(11) DEFAULT NULL,
  `Dest` char(5) DEFAULT NULL,
  `DestCityName` varchar(100) DEFAULT NULL,
  `DestState` char(2) DEFAULT NULL,
  `DestStateFips` varchar(10) DEFAULT NULL,
  `DestStateName` varchar(100) DEFAULT NULL,
  `DestWac` int(11) DEFAULT NULL,
  `CRSDepTime` int(11) DEFAULT NULL,
  `DepTime` int(11) DEFAULT NULL,
  `DepDelay` int(11) DEFAULT NULL,
  `DepDelayMinutes` int(11) DEFAULT NULL,
  `DepDel15` int(11) DEFAULT NULL,
  `DepartureDelayGroups` int(11) DEFAULT NULL,
  `DepTimeBlk` varchar(20) DEFAULT NULL,
  `TaxiOut` int(11) DEFAULT NULL,
  `WheelsOff` int(11) DEFAULT NULL,
  `WheelsOn` int(11) DEFAULT NULL,
  `TaxiIn` int(11) DEFAULT NULL,
  `CRSArrTime` int(11) DEFAULT NULL,
  `ArrTime` int(11) DEFAULT NULL,
  `ArrDelay` int(11) DEFAULT NULL,
  `ArrDelayMinutes` int(11) DEFAULT NULL,
  `ArrDel15` int(11) DEFAULT NULL,
  `ArrivalDelayGroups` int(11) DEFAULT NULL,
  `ArrTimeBlk` varchar(20) DEFAULT NULL,
  `Cancelled` tinyint(4) DEFAULT NULL,
  `CancellationCode` char(1) DEFAULT NULL,
  `Diverted` tinyint(4) DEFAULT NULL,
  `CRSElapsedTime` int(11) DEFAULT NULL,
  `ActualElapsedTime` int(11) DEFAULT NULL,
  `AirTime` int(11) DEFAULT NULL,
  `Flights` int(11) DEFAULT NULL,
  `Distance` int(11) DEFAULT NULL,
  `DistanceGroup` tinyint(4) DEFAULT NULL,
  `CarrierDelay` int(11) DEFAULT NULL,
  `WeatherDelay` int(11) DEFAULT NULL,
  `NASDelay` int(11) DEFAULT NULL,
  `SecurityDelay` int(11) DEFAULT NULL,
  `LateAircraftDelay` int(11) DEFAULT NULL,
  `FirstDepTime` varchar(10) DEFAULT NULL,
  `TotalAddGTime` varchar(10) DEFAULT NULL,
  `LongestAddGTime` varchar(10) DEFAULT NULL,
  `DivAirportLandings` varchar(10) DEFAULT NULL,
  `DivReachedDest` varchar(10) DEFAULT NULL,
  `DivActualElapsedTime` varchar(10) DEFAULT NULL,
  `DivArrDelay` varchar(10) DEFAULT NULL,
  `DivDistance` varchar(10) DEFAULT NULL,
  `Div1Airport` varchar(10) DEFAULT NULL,
  `Div1AirportID` int(11) DEFAULT NULL,
  `Div1AirportSeqID` int(11) DEFAULT NULL,
  `Div1WheelsOn` varchar(10) DEFAULT NULL,
  `Div1TotalGTime` varchar(10) DEFAULT NULL,
  `Div1LongestGTime` varchar(10) DEFAULT NULL,
  `Div1WheelsOff` varchar(10) DEFAULT NULL,
  `Div1TailNum` varchar(10) DEFAULT NULL,
  `Div2Airport` varchar(10) DEFAULT NULL,
  `Div2AirportID` int(11) DEFAULT NULL,
  `Div2AirportSeqID` int(11) DEFAULT NULL,
  `Div2WheelsOn` varchar(10) DEFAULT NULL,
  `Div2TotalGTime` varchar(10) DEFAULT NULL,
  `Div2LongestGTime` varchar(10) DEFAULT NULL,
  `Div2WheelsOff` varchar(10) DEFAULT NULL,
  `Div2TailNum` varchar(10) DEFAULT NULL,
  `Div3Airport` varchar(10) DEFAULT NULL,
  `Div3AirportID` int(11) DEFAULT NULL,
  `Div3AirportSeqID` int(11) DEFAULT NULL,
  `Div3WheelsOn` varchar(10) DEFAULT NULL,
  `Div3TotalGTime` varchar(10) DEFAULT NULL,
  `Div3LongestGTime` varchar(10) DEFAULT NULL,
  `Div3WheelsOff` varchar(10) DEFAULT NULL,
  `Div3TailNum` varchar(10) DEFAULT NULL,
  `Div4Airport` varchar(10) DEFAULT NULL,
  `Div4AirportID` int(11) DEFAULT NULL,
  `Div4AirportSeqID` int(11) DEFAULT NULL,
  `Div4WheelsOn` varchar(10) DEFAULT NULL,
  `Div4TotalGTime` varchar(10) DEFAULT NULL,
  `Div4LongestGTime` varchar(10) DEFAULT NULL,
  `Div4WheelsOff` varchar(10) DEFAULT NULL,
  `Div4TailNum` varchar(10) DEFAULT NULL,
  `Div5Airport` varchar(10) DEFAULT NULL,
  `Div5AirportID` int(11) DEFAULT NULL,
  `Div5AirportSeqID` int(11) DEFAULT NULL,
  `Div5WheelsOn` varchar(10) DEFAULT NULL,
  `Div5TotalGTime` varchar(10) DEFAULT NULL,
  `Div5LongestGTime` varchar(10) DEFAULT NULL,
  `Div5WheelsOff` varchar(10) DEFAULT NULL,
  `Div5TailNum` varchar(10) DEFAULT NULL,
  `timestamp_tidb` timestamp default current_timestamp,
  `timestamp_mysql` timestamp default current_timestamp
) DEFAULT CHARSET=latin1 ;
