resource "aws_iam_role" "api_role_instagame_launcher" {
  assume_role_policy =  <<EOF
{
 "Version": "2012-10-17",
 "Statement": [
   {
     "Action": "sts:AssumeRole",
     "Principal": {
       "Service": "lambda.amazonaws.com"
     },
     "Effect": "Allow",
     "Sid": ""
   }
 ]
}
EOF
}

resource "aws_iam_policy" "iam_policy_instagame_launcher_lambda" {
 name         = "iam_policy_for_instagame_launcher_lambda"
 policy = <<EOF
{
 "Version": "2012-10-17",
 "Statement": [
   {
     "Action": [
       "ecs:ListServices",
       "ecs:ListTasks",
       "ecs:DescribeTasks",
       "ecs:UpdateService",
       "ec2:DescribeNetworkInterfaces"
     ],
     "Resource": "*",
     "Effect": "Allow"
   }
 ]
}
EOF
}

resource "aws_iam_role_policy_attachment" "launcher_api_iam_role_policy_attachment" {
  role       = aws_iam_role.api_role_instagame_launcher.name
  policy_arn = aws_iam_policy.iam_policy_instagame_launcher_lambda.arn
}
