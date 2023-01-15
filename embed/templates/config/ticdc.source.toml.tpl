[sink]                                                                                                                                                                                                      
dispatchers = [
  {matcher = ['*.*'], topic = "{schema}_{table}", partition ="ts"},
]
