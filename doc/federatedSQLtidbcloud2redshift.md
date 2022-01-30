# Create Basic Resource
## VPC
  Create the VPC as below screenshot
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/01.01.vpc.png"></kbd>
## VPC Peering
### Invite the client VPC from TiDB cloud side
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/01.02.tidbcloud.vpcpeering.01.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/01.02.tidbcloud.vpcpeering.02.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/01.02.tidbcloud.vpcpeering.03.png"></kbd>
### Accept the vpc peering invitation.
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/01.02.tidbcloud.vpcpeering.04.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/01.02.tidbcloud.vpcpeering.05.png"></kbd>
### Check the vpc peering status after several minutes
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/01.02.tidbcloud.vpcpeering.06.png"></kbd>
## Subnets
  Create the subnets as below screenshot.
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/01.03.subnets.png"></kbd>
## Route Table
  Create the route table as below screenshots.
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/01.04.route.01.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/01.04.route.02.png"></kbd>
## Create endpoint to allow redshift to access secretsmanager
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/01.05.endpoint.01.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/01.05.endpoint.02.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/01.05.endpoint.03.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/01.05.endpoint.04.png"></kbd>
  
# Create TiDB Cloud secret resource
## Create secret resource
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.01.secrets.01.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.01.secrets.02.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.01.secrets.03.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.01.secrets.04.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.01.secrets.05.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.01.secrets.06.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.01.secrets.07.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.01.secrets.08.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.01.secrets.09.png"></kbd>
  
## Create policy to access TiDB Cloud access secrets
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.02.policy.01.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.02.policy.02.png"></kbd>


    
    {
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "AccessSecret",
            "Effect": "Allow",
            "Action": [
                "secretsmanager:GetResourcePolicy",
                "secretsmanager:GetSecretValue",
                "secretsmanager:DescribeSecret",
                "secretsmanager:ListSecretVersionIds"
            ],
            "Resource": "arn:aws:secretsmanager:ap-northeast-1:xxxxxxxxxxxx:secret:tidbcloud_secret-Nj3mzF"
        },
        {
            "Sid": "VisualEditor1",
            "Effect": "Allow",
            "Action": [
                "secretsmanager:GetRandomPassword",
                "secretsmanager:ListSecrets"
            ],
            "Resource": "*"
        }
    ]
}

  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.02.policy.03.png"></kbd>

  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.02.policy.04.png"></kbd>

  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.02.policy.05.png"></kbd>

  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.02.policy.06.png"></kbd>
  
## Create role to access TiDB Cloud access secrets
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.03.role.01.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.03.role.02.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.03.role.03.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.03.role.04.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.03.role.05.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.03.role.06.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.03.role.07.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/02.03.role.08.png"></kbd>
  
# Create redshift resource
## Create the redshift subnets group
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.01.subnetgroup.01.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.01.subnetgroup.02.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.01.subnetgroup.03.png"></kbd>
## Create the redshift and federated query to access TiDB Cloud
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.02.redshift.01.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.02.redshift.02.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.02.redshift.03.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.02.redshift.04.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.02.redshift.05.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.02.redshift.06.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.02.redshift.07.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.02.redshift.08.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.02.redshift.09.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.02.redshift.10.png"></kbd>
 
    CREATE EXTERNAL SCHEMA tidbcloud_schema
    FROM MYSQL
    DATABASE 'test' URI 'private-tidb.3072164e.fc69e292.ap-northeast-1.prod.aws.tidbcloud.com' 
    PORT 4000
    IAM_ROLE 'arn:aws:iam::xxxxxxxxxxxx:role/tidbcloud-access-role'
    SECRET_ARN 'arn:aws:secretsmanager:ap-northeast-1:xxxxxxxxxxxx:secret:tidbcloud-conninfo-DrM3Zm';
 
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.02.redshift.11.png"></kbd>
 
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.02.redshift.12.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.02.redshift.13.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.02.redshift.14.png"></kbd>
  
  <kbd><img src="https://github.com/luyomo/OhMyTiUP/blob/main/doc/png/federatedSQLtidbcloud2redshift/03.02.redshift.15.png"></kbd>
