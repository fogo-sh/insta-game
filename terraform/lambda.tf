data "archive_file" "lambda" {
  type        = "zip"
  source_file = "../lambda/launcher/launcher.py"
  output_path = "lambda.zip"
}

resource "aws_lambda_function" "lambda_function_instagame" {
  filename      = data.archive_file.lambda.output_path
  function_name = "launcher_function"
  role          = aws_iam_role.api_role_instagame_launcher.arn
  handler       = "launcher.handler"
  runtime       = "python3.8"
  timeout       = 120
  source_code_hash = filebase64sha256(data.archive_file.lambda.output_path)
  publish          = true
}

resource "aws_lambda_function_url" "test_latest" {
  function_name      = aws_lambda_function.lambda_function_instagame.function_name
  authorization_type = "NONE"
}

output "prod_url" {
  value = aws_lambda_function_url.test_latest.function_url
}
