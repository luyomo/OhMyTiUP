CREATE TABLE `ontime` (
     id bigint primary key auto_increment,
    `Year`                            int(11),
    `Quarter`                         int(11),
    `Month`                           int(11),
    `DayofMonth`                      int(11),
    `DayOfWeek`                       int(11),
    `FlightDate`                      Date,
    `Reporting_Airline`               varchar(32),
    `DOT_ID_Reporting_Airline`        int(11),
    `IATA_CODE_Reporting_Airline`     varchar(32),
    `Tail_Number`                     varchar(32),
    `Flight_Number_Reporting_Airline` varchar(32),
    `OriginAirportID`                 int(11),
    `OriginAirportSeqID`              int(11),
    `OriginCityMarketID`              int(11),
    `Origin`                          char(5),
    `OriginCityName`                  varchar(32),
    `OriginState`                     char(2),
    `OriginStateFips`                 char(2),
    `OriginStateName`                 varchar(32),
    `OriginWac`                       int(11),
    `DestAirportID`                   int(11),
    `DestAirportSeqID`                int(11),
    `DestCityMarketID`                int(11),
    `Dest`                            char(5),
    `DestCityName`                    varchar(32),
    `DestState`                       char(2),
    `DestStateFips`                   char(2),
    `DestStateName`                   varchar(32),
    `DestWac`                         int(11),
    `CRSDepTime`                      int(11),
    `DepTime`                         int(11),
    `DepDelay`                        int(11),
    `DepDelayMinutes`                 int(11),
    `DepDel15`                        int(11),
    `DepartureDelayGroups`            varchar(32),
    `DepTimeBlk`                      varchar(32),
    `TaxiOut`                         int(11),
    `WheelsOff`                       varchar(32),
    `WheelsOn`                        varchar(32),
    `TaxiIn`                          int(11),
    `CRSArrTime`                      int(11),
    `ArrTime`                         int(11),
    `ArrDelay`                        int(11),
    `ArrDelayMinutes`                 int(11),
    `ArrDel15`                        int(11),
    `ArrivalDelayGroups`              varchar(32),
    `ArrTimeBlk`                      varchar(32),
    `Cancelled`                       int(11),
    `CancellationCode`                char(1),
    `Diverted`                        int(11),
    `CRSElapsedTime`                  int(11),
    `ActualElapsedTime`               int(11),
    `AirTime`                         int(11),
    `Flights`                         int(11),
    `Distance`                        int(11),
    `DistanceGroup`                   int(11),
    `CarrierDelay`                    int(11),
    `WeatherDelay`                    int(11),
    `NASDelay`                        int(11),
    `SecurityDelay`                   int(11),
    `LateAircraftDelay`               int(11),
    `FirstDepTime`                    int(11),
    `TotalAddGTime`                   int(11),
    `LongestAddGTime`                 int(11),
    `DivAirportLandings`              int(11),
    `DivReachedDest`                  int(11),
    `DivActualElapsedTime`            int(11),
    `DivArrDelay`                     int(11),
    `DivDistance`                     int(11),
    `Div1Airport`                     varchar(32),
    `Div1AirportID`                   int(11),
    `Div1AirportSeqID`                int(11),
    `Div1WheelsOn`                    int(11),
    `Div1TotalGTime`                  int(11),
    `Div1LongestGTime`                int(11),
    `Div1WheelsOff`                   int(11),
    `Div1TailNum`                     varchar(32),
    `Div2Airport`                     varchar(32),
    `Div2AirportID`                   int(11),
    `Div2AirportSeqID`                int(11),
    `Div2WheelsOn`                    int(11),
    `Div2TotalGTime`                  int(11),
    `Div2LongestGTime`                int(11),
    `Div2WheelsOff`                   int(11),
    `Div2TailNum`                     varchar(32),
    `Div3Airport`                     varchar(32),
    `Div3AirportID`                   int(11),
    `Div3AirportSeqID`                int(11),
    `Div3WheelsOn`                    int(11),
    `Div3TotalGTime`                  int(11),
    `Div3LongestGTime`                int(11),
    `Div3WheelsOff`                   int(11),
    `Div3TailNum`                     varchar(32),
    `Div4Airport`                     varchar(32),
    `Div4AirportID`                   int(11),
    `Div4AirportSeqID`                int(11),
    `Div4WheelsOn`                    int(11),
    `Div4TotalGTime`                  int(11),
    `Div4LongestGTime`                int(11),
    `Div4WheelsOff`                   int(11),
    `Div4TailNum`                     varchar(32),
    `Div5Airport`                     varchar(32),
    `Div5AirportID`                   int(11),
    `Div5AirportSeqID`                int(11),
    `Div5WheelsOn`                    int(11),
    `Div5TotalGTime`                  int(11),
    `Div5LongestGTime`                int(11),
    `Div5WheelsOff`                   int(11),
    `Div5TailNum`                     varchar(32),
    `timestamp_tidb` timestamp default current_timestamp,
    `timestamp_mysql` timestamp default current_timestamp
)
 DEFAULT CHARSET=latin1 ;
