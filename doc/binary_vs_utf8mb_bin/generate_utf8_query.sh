#!/bin/bash
for encode in 0x0 0x10 0x20 0x30 0x40 0x50 0x60 0x70 0xc280 0xc290 0xc2a0 0xc2b0 0xc380 0xc390 0xc3a0 0xc3b0 0xc480 0xc490 0xc4a0 0xc4b0 0xc580 0xc590 0xc5a0 0xc5b0 0xc680 0xc690 0xc6a0 0xc6b0 0xc780 0xc790 0xc7a0 0xc7b0 0xc880 0xc890 0xc8a0 0xc8b0 0xc980 0xc990 0xc9a0 0xc9b0 0xca80 0xca90 0xcaa0 0xcab0 0xcb80 0xcb90 0xcba0 0xcbb0 0xcc80 0xcc90 0xcca0 0xccb0 0xcd80 0xcd90 0xcda0 0xcdb0 0xce80 0xce90 0xcea0 0xceb0 0xcf80 0xcf90 0xcfa0 0xcfb0 0xd080 0xd090 0xd0a0 0xd0b0 0xd180 0xd190 0xd1a0 0xd1b0 0xd280 0xd290 0xd2a0 0xd2b0 0xd380 0xd390 0xd3a0 0xd3b0 0xd480 0xd490 0xd4a0 0xd4b0 0xd580 0xd590 0xd5a0 0xd5b0 0xd680 0xd690 0xd6a0 0xd6b0 0xd780 0xd790 0xd7a0 0xd7b0 0xd880 0xd890 0xd8a0 0xd8b0 0xd980 0xd990 0xd9a0 0xd9b0 0xda80 0xda90 0xdaa0 0xdab0 0xdb80 0xdb90 0xdba0 0xdbb0 0xdc80 0xdc90 0xdca0 0xdcb0 0xdd80 0xdd90 0xdda0 0xddb0 0xde80 0xde90 0xdea0 0xdeb0 0xdf80 0xdf90 0xdfa0 0xdfb0 0xe0a080 0xe0a090 0xe0a0a0 0xe0a0b0 0xe0a180 0xe0a190 0xe0a1a0 0xe0a1b0 0xe0a280 0xe0a290 0xe0a2a0 0xe0a2b0 0xe0a380 0xe0a390 0xe0a3a0 0xe0a3b0 0xe0a480 0xe0a490 0xe0a4a0 0xe0a4b0 0xe0a580 0xe0a590 0xe0a5a0 0xe0a5b0 0xe0a680 0xe0a690 0xe0a6a0 0xe0a6b0 0xe0a780 0xe0a790 0xe0a7a0 0xe0a7b0 0xe0a880 0xe0a890 0xe0a8a0 0xe0a8b0 0xe0a980 0xe0a990 0xe0a9a0 0xe0a9b0 0xe0aa80 0xe0aa90 0xe0aaa0 0xe0aab0 0xe0ab80 0xe0ab90 0xe0aba0 0xe0abb0 0xe0ac80 0xe0ac90 0xe0aca0 0xe0acb0 0xe0ad80 0xe0ad90 0xe0ada0 0xe0adb0 0xe0ae80 0xe0ae90 0xe0aea0 0xe0aeb0 0xe0af80 0xe0af90 0xe0afa0 0xe0afb0 0xe0b080 0xe0b090 0xe0b0a0 0xe0b0b0 0xe0b180 0xe0b190 0xe0b1a0 0xe0b1b0 0xe0b280 0xe0b290 0xe0b2a0 0xe0b2b0 0xe0b380 0xe0b390 0xe0b3a0 0xe0b3b0 0xe0b480 0xe0b490 0xe0b4a0 0xe0b4b0 0xe0b580 0xe0b590 0xe0b5a0 0xe0b5b0 0xe0b680 0xe0b690 0xe0b6a0 0xe0b6b0 0xe0b780 0xe0b790 0xe0b7a0 0xe0b7b0 0xe0b880 0xe0b890 0xe0b8a0 0xe0b8b0 0xe0b980 0xe0b990 0xe0b9a0 0xe0b9b0 0xe0ba80 0xe0ba90 0xe0baa0 0xe0bab0 0xe0bb80 0xe0bb90 0xe0bba0 0xe0bbb0 0xe0bc80 0xe0bc90 0xe0bca0 0xe0bcb0 0xe0bd80 0xe0bd90 0xe0bda0 0xe0bdb0 0xe0be80 0xe0be90 0xe0bea0 0xe0beb0 0xe0bf80 0xe0bf90 0xe0bfa0 0xe0bfb0 0xe18080 0xe18090 0xe180a0 0xe180b0 0xe18180 0xe18190 0xe181a0 0xe181b0 0xe18280 0xe18290 0xe182a0 0xe182b0 0xe18380 0xe18390 0xe183a0 0xe183b0 0xe18480 0xe18490 0xe184a0 0xe184b0 0xe18580 0xe18590 0xe185a0 0xe185b0 0xe18680 0xe18690 0xe186a0 0xe186b0 0xe18780 0xe18790 0xe187a0 0xe187b0 0xe18880 0xe18890 0xe188a0 0xe188b0 0xe18980 0xe18990 0xe189a0 0xe189b0 0xe18a80 0xe18a90 0xe18aa0 0xe18ab0 0xe18b80 0xe18b90 0xe18ba0 0xe18bb0 0xe18c80 0xe18c90 0xe18ca0 0xe18cb0 0xe18d80 0xe18d90 0xe18da0 0xe18db0 0xe18e80 0xe18e90 0xe18ea0 0xe18eb0 0xe18f80 0xe18f90 0xe18fa0 0xe18fb0 0xe19080 0xe19090 0xe190a0 0xe190b0 0xe19180 0xe19190 0xe191a0 0xe191b0 0xe19280 0xe19290 0xe192a0 0xe192b0 0xe19380 0xe19390 0xe193a0 0xe193b0 0xe19480 0xe19490 0xe194a0 0xe194b0 0xe19580 0xe19590 0xe195a0 0xe195b0 0xe19680 0xe19690 0xe196a0 0xe196b0 0xe19780 0xe19790 0xe197a0 0xe197b0 0xe19880 0xe19890 0xe198a0 0xe198b0 0xe19980 0xe19990 0xe199a0 0xe199b0 0xe19a80 0xe19a90 0xe19aa0 0xe19ab0 0xe19b80 0xe19b90 0xe19ba0 0xe19bb0 0xe19c80 0xe19c90 0xe19ca0 0xe19cb0 0xe19d80 0xe19d90 0xe19da0 0xe19db0 0xe19e80 0xe19e90 0xe19ea0 0xe19eb0 0xe19f80 0xe19f90 0xe19fa0 0xe19fb0 0xe1a080 0xe1a090 0xe1a0a0 0xe1a0b0 0xe1a180 0xe1a190 0xe1a1a0 0xe1a1b0 0xe1a280 0xe1a290 0xe1a2a0 0xe1a2b0 0xe1a380 0xe1a390 0xe1a3a0 0xe1a3b0 0xe1a480 0xe1a490 0xe1a4a0 0xe1a4b0 0xe1a580 0xe1a590 0xe1a5a0 0xe1a5b0 0xe1a680 0xe1a690 0xe1a6a0 0xe1a6b0 0xe1a780 0xe1a790 0xe1a7a0 0xe1a7b0 0xe1a880 0xe1a890 0xe1a8a0 0xe1a8b0 0xe1a980 0xe1a990 0xe1a9a0 0xe1a9b0 0xe1aa80 0xe1aa90 0xe1aaa0 0xe1aab0 0xe1ab80 0xe1ab90 0xe1aba0 0xe1abb0 0xe1ac80 0xe1ac90 0xe1aca0 0xe1acb0 0xe1ad80 0xe1ad90 0xe1ada0 0xe1adb0 0xe1ae80 0xe1ae90 0xe1aea0 0xe1aeb0 0xe1af80 0xe1af90 0xe1afa0 0xe1afb0 0xe1b080 0xe1b090 0xe1b0a0 0xe1b0b0 0xe1b180 0xe1b190 0xe1b1a0 0xe1b1b0 0xe1b280 0xe1b290 0xe1b2a0 0xe1b2b0 0xe1b380 0xe1b390 0xe1b3a0 0xe1b3b0 0xe1b480 0xe1b490 0xe1b4a0 0xe1b4b0 0xe1b580 0xe1b590 0xe1b5a0 0xe1b5b0 0xe1b680 0xe1b690 0xe1b6a0 0xe1b6b0 0xe1b780 0xe1b790 0xe1b7a0 0xe1b7b0 0xe1b880 0xe1b890 0xe1b8a0 0xe1b8b0 0xe1b980 0xe1b990 0xe1b9a0 0xe1b9b0 0xe1ba80 0xe1ba90 0xe1baa0 0xe1bab0 0xe1bb80 0xe1bb90 0xe1bba0 0xe1bbb0 0xe1bc80 0xe1bc90 0xe1bca0 0xe1bcb0 0xe1bd80 0xe1bd90 0xe1bda0 0xe1bdb0 0xe1be80 0xe1be90 0xe1bea0 0xe1beb0 0xe1bf80 0xe1bf90 0xe1bfa0 0xe1bfb0 0xe28080 0xe28090 0xe280a0 0xe280b0 0xe28180 0xe28190 0xe281a0 0xe281b0 0xe28280 0xe28290 0xe282a0 0xe282b0 0xe28380 0xe28390 0xe283a0 0xe283b0 0xe28480 0xe28490 0xe284a0 0xe284b0 0xe28580 0xe28590 0xe285a0 0xe285b0 0xe28680 0xe28690 0xe286a0 0xe286b0 0xe28780 0xe28790 0xe287a0 0xe287b0 0xe28880 0xe28890 0xe288a0 0xe288b0 0xe28980 0xe28990 0xe289a0 0xe289b0 0xe28a80 0xe28a90 0xe28aa0 0xe28ab0 0xe28b80 0xe28b90 0xe28ba0 0xe28bb0 0xe28c80 0xe28c90 0xe28ca0 0xe28cb0 0xe28d80 0xe28d90 0xe28da0 0xe28db0 0xe28e80 0xe28e90 0xe28ea0 0xe28eb0 0xe28f80 0xe28f90 0xe28fa0 0xe28fb0 0xe29080 0xe29090 0xe290a0 0xe290b0 0xe29180 0xe29190 0xe291a0 0xe291b0 0xe29280 0xe29290 0xe292a0 0xe292b0 0xe29380 0xe29390 0xe293a0 0xe293b0 0xe29480 0xe29490 0xe294a0 0xe294b0 0xe29580 0xe29590 0xe295a0 0xe295b0 0xe29680 0xe29690 0xe296a0 0xe296b0 0xe29780 0xe29790 0xe297a0 0xe297b0 0xe29880 0xe29890 0xe298a0 0xe298b0 0xe29980 0xe29990 0xe299a0 0xe299b0 0xe29a80 0xe29a90 0xe29aa0 0xe29ab0 0xe29b80 0xe29b90 0xe29ba0 0xe29bb0 0xe29c80 0xe29c90 0xe29ca0 0xe29cb0 0xe29d80 0xe29d90 0xe29da0 0xe29db0 0xe29e80 0xe29e90 0xe29ea0 0xe29eb0 0xe29f80 0xe29f90 0xe29fa0 0xe29fb0 0xe2a080 0xe2a090 0xe2a0a0 0xe2a0b0 0xe2a180 0xe2a190 0xe2a1a0 0xe2a1b0 0xe2a280 0xe2a290 0xe2a2a0 0xe2a2b0 0xe2a380 0xe2a390 0xe2a3a0 0xe2a3b0 0xe2a480 0xe2a490 0xe2a4a0 0xe2a4b0 0xe2a580 0xe2a590 0xe2a5a0 0xe2a5b0 0xe2a680 0xe2a690 0xe2a6a0 0xe2a6b0 0xe2a780 0xe2a790 0xe2a7a0 0xe2a7b0 0xe2a880 0xe2a890 0xe2a8a0 0xe2a8b0 0xe2a980 0xe2a990 0xe2a9a0 0xe2a9b0 0xe2aa80 0xe2aa90 0xe2aaa0 0xe2aab0 0xe2ab80 0xe2ab90 0xe2aba0 0xe2abb0 0xe2ac80 0xe2ac90 0xe2aca0 0xe2acb0 0xe2ad80 0xe2ad90 0xe2ada0 0xe2adb0 0xe2ae80 0xe2ae90 0xe2aea0 0xe2aeb0 0xe2af80 0xe2af90 0xe2afa0 0xe2afb0 0xe2b080 0xe2b090 0xe2b0a0 0xe2b0b0 0xe2b180 0xe2b190 0xe2b1a0 0xe2b1b0 0xe2b280 0xe2b290 0xe2b2a0 0xe2b2b0 0xe2b380 0xe2b390 0xe2b3a0 0xe2b3b0 0xe2b480 0xe2b490 0xe2b4a0 0xe2b4b0 0xe2b580 0xe2b590 0xe2b5a0 0xe2b5b0 0xe2b680 0xe2b690 0xe2b6a0 0xe2b6b0 0xe2b780 0xe2b790 0xe2b7a0 0xe2b7b0 0xe2b880 0xe2b890 0xe2b8a0 0xe2b8b0 0xe2b980 0xe2b990 0xe2b9a0 0xe2b9b0 0xe2ba80 0xe2ba90 0xe2baa0 0xe2bab0 0xe2bb80 0xe2bb90 0xe2bba0 0xe2bbb0 0xe2bc80 0xe2bc90 0xe2bca0 0xe2bcb0 0xe2bd80 0xe2bd90 0xe2bda0 0xe2bdb0 0xe2be80 0xe2be90 0xe2bea0 0xe2beb0 0xe2bf80 0xe2bf90 0xe2bfa0 0xe2bfb0 0xe38080 0xe38090 0xe380a0 0xe380b0 0xe38180 0xe38190 0xe381a0 0xe381b0 0xe38280 0xe38290 0xe382a0 0xe382b0 0xe38380 0xe38390 0xe383a0 0xe383b0 0xe38480 0xe38490 0xe384a0 0xe384b0 0xe38580 0xe38590 0xe385a0 0xe385b0 0xe38680 0xe38690 0xe386a0 0xe386b0 0xe38780 0xe38790 0xe387a0 0xe387b0 0xe38880 0xe38890 0xe388a0 0xe388b0 0xe38980 0xe38990 0xe389a0 0xe389b0 0xe38a80 0xe38a90 0xe38aa0 0xe38ab0 0xe38b80 0xe38b90 0xe38ba0 0xe38bb0 0xe38c80 0xe38c90 0xe38ca0 0xe38cb0 0xe38d80 0xe38d90 0xe38da0 0xe38db0 0xe38e80 0xe38e90 0xe38ea0 0xe38eb0 0xe38f80 0xe38f90 0xe38fa0 0xe38fb0 0xe39080 0xe39090 0xe390a0 0xe390b0 0xe39180 0xe39190 0xe391a0 0xe391b0 0xe39280 0xe39290 0xe392a0 0xe392b0 0xe39380 0xe39390 0xe393a0 0xe393b0 0xe39480 0xe39490 0xe394a0 0xe394b0 0xe39580 0xe39590 0xe395a0 0xe395b0 0xe39680 0xe39690 0xe396a0 0xe396b0 0xe39780 0xe39790 0xe397a0 0xe397b0 0xe39880 0xe39890 0xe398a0 0xe398b0 0xe39980 0xe39990 0xe399a0 0xe399b0 0xe39a80 0xe39a90 0xe39aa0 0xe39ab0 0xe39b80 0xe39b90 0xe39ba0 0xe39bb0 0xe39c80 0xe39c90 0xe39ca0 0xe39cb0 0xe39d80 0xe39d90 0xe39da0 0xe39db0 0xe39e80 0xe39e90 0xe39ea0 0xe39eb0 0xe39f80 0xe39f90 0xe39fa0 0xe39fb0 0xe3a080 0xe3a090 0xe3a0a0 0xe3a0b0 0xe3a180 0xe3a190 0xe3a1a0 0xe3a1b0 0xe3a280 0xe3a290 0xe3a2a0 0xe3a2b0 0xe3a380 0xe3a390 0xe3a3a0 0xe3a3b0 0xe3a480 0xe3a490 0xe3a4a0 0xe3a4b0 0xe3a580 0xe3a590 0xe3a5a0 0xe3a5b0 0xe3a680 0xe3a690 0xe3a6a0 0xe3a6b0 0xe3a780 0xe3a790 0xe3a7a0 0xe3a7b0 0xe3a880 0xe3a890 0xe3a8a0 0xe3a8b0 0xe3a980 0xe3a990 0xe3a9a0 0xe3a9b0 0xe3aa80 0xe3aa90 0xe3aaa0 0xe3aab0 0xe3ab80 0xe3ab90 0xe3aba0 0xe3abb0 0xe3ac80 0xe3ac90 0xe3aca0 0xe3acb0 0xe3ad80 0xe3ad90 0xe3ada0 0xe3adb0 0xe3ae80 0xe3ae90 0xe3aea0 0xe3aeb0 0xe3af80 0xe3af90 0xe3afa0 0xe3afb0 0xe3b080 0xe3b090 0xe3b0a0 0xe3b0b0 0xe3b180 0xe3b190 0xe3b1a0 0xe3b1b0 0xe3b280 0xe3b290 0xe3b2a0 0xe3b2b0 0xe3b380 0xe3b390 0xe3b3a0 0xe3b3b0 0xe3b480 0xe3b490 0xe3b4a0 0xe3b4b0 0xe3b580 0xe3b590 0xe3b5a0 0xe3b5b0 0xe3b680 0xe3b690 0xe3b6a0 0xe3b6b0 0xe3b780 0xe3b790 0xe3b7a0 0xe3b7b0 0xe3b880 0xe3b890 0xe3b8a0 0xe3b8b0 0xe3b980 0xe3b990 0xe3b9a0 0xe3b9b0 0xe3ba80 0xe3ba90 0xe3baa0 0xe3bab0 0xe3bb80 0xe3bb90 0xe3bba0 0xe3bbb0 0xe3bc80 0xe3bc90 0xe3bca0 0xe3bcb0 0xe3bd80 0xe3bd90 0xe3bda0 0xe3bdb0 0xe3be80 0xe3be90 0xe3bea0 0xe3beb0 0xe3bf80 0xe3bf90 0xe3bfa0 0xe3bfb0
do
    for i in $(seq 0 15);
    do printf 'insert into binary_vs_utf8mb4_bin (col_binary, col_utf8mb4_bin) values ( CAST(0x%x AS CHAR),  CAST(0x%x AS CHAR) ); \n' $(($encode+i)) $(($encode+i));
    done
done