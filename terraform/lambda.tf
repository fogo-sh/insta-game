data "archive_file" "lambda" {
  type        = "zip"
  source_file = "../lambda/launcher/launcher.py"
  output_path = "lambda.zip"
}

resource "aws_lambda_function" "launcher_function" {
  filename      = data.archive_file.lambda.output_path
  function_name = "launcher_function"
  role          = aws_iam_role.launcher_api_role.arn
  handler       = "launcher.handler"
  runtime       = "python3.8"
  timeout       = 120

  source_code_hash = filebase64sha256(data.archive_file.lambda.output_path)
  publish          = true
}
