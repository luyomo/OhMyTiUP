import sys
import logging
import pymysql
import subprocess
import boto3
import os
import sqlalchemy
import psycopg2
from sqlalchemy import create_engine
from sqlalchemy.orm import scoped_session, sessionmaker
from datetime import datetime,timedelta

logger = logging.getLogger()
logger.setLevel(logging.INFO)

def lambda_handler(event, context):
    s3_region = os.environ['S3_REGION']
    s3_bucket_name = os.environ['BUCKET_NAME']
    s3_data_folder = os.environ['S3_DATA_FOLDER']
    s3_lib_folder = os.environ['S3_LIB_FOLDER']

    tidb_host = os.environ['TiDB_HOST']
    tidb_port = os.environ['TiDB_PORT']
    tidb_user = os.environ['TiDB_USER']
    tidb_pass = os.environ['TiDB_PASS']

    rd_host = os.environ['RD_HOST']
    rd_port = os.environ['RD_PORT']
    rd_user = os.environ['RD_USER']
    rd_pass = os.environ['RD_PASS']
    rd_name = os.environ['RD_NAME']

    rd_role = os.environ['REDSHIFT_ROLE']

    # Export data from TiDB Cloud to S3
    exportData(s3_bucket_name, s3_lib_folder, tidb_host, tidb_port, tidb_user, tidb_pass, s3_data_folder, s3_region )

    # Loop all the resources
    s3resource = boto3.resource('s3')
    bucket = s3resource.Bucket(s3_bucket_name)
    for obj in bucket.objects.filter(Prefix=s3_data_folder + "/test.test01"):
        print(obj.key)
        if obj.key.split(".")[-1] == "csv":
            # Import data to redshift
            importData(obj,rd_host,rd_port,rd_user,rd_pass,rd_name, s3_region, rd_role)

    return "Completed sucessfully"

def exportData(s3_bucket_name, s3_lib_folder, tidb_host, tidb_port, tidb_user, tidb_pass, s3_data_folder, s3_region ):
    work_dir = '/tmp'

    # Download bin file from S3
    s3 = boto3.client('s3')
    s3.download_file(s3_bucket_name, s3_lib_folder + "/dumpling", work_dir + '/dumpling' )
    ret = os.path.isfile("/tmp/dumpling")
    if ret is False:
        print(ret)
        return 1

    cmd = 'chmod +x /tmp/dumpling'
    ret = os.system(cmd)
    if ret is False:
        print('chmod dumpling failed')
        return 2

    # Dumpling data from TiDB Cloud
    try:
        subprocess.run(["/tmp/dumpling", "-u", tidb_user, "-P", tidb_port, "-h", tidb_host, "-p" + tidb_pass, "--filetype", "csv", "-o",  "s3://" + s3_bucket_name + "/" + s3_data_folder, "--s3.region", s3_region])
    except subprocess.CalledProcessError as e:
        print(e.output)
        logger.info(e.output)

def importData(fileName, rdhost, rdport, rduser, rdpass, rdname, s3_region, rd_role):
    print(fileName)

    DBC = "postgresql://{0}:{1}@{2}:{3}/{4}".format(rduser, rdpass, rdhost, rdport, rdname)
    arrFileName = fileName.key.split('/')[1]
    tableName = arrFileName.split(".")[1]

    DELIMITER = "','"

    engine = create_engine(DBC)
    db = scoped_session(sessionmaker(bind=engine))
    # Send files from S3 into redshift
    copy_query = "COPY {0} from 's3://{1}/{2}' iam_role '{3}' delimiter {4} IGNOREHEADER 1 REGION '{5}'".format(tableName, fileName.bucket_name, fileName.key, rd_role, DELIMITER, s3_region)

    db.execute(copy_query)
    db.commit()
    db.close()
