resource "aws_api_gateway_rest_api" "launcher_api" {
  name        = "LauncherAPI"
  description = "Example Rest Api"
}

resource "aws_api_gateway_resource" "launcher_api_resource" {
  rest_api_id = aws_api_gateway_rest_api.launcher_api.id
  parent_id   = aws_api_gateway_rest_api.launcher_api.root_resource_id
  path_part   = "launch"
}

resource "aws_api_gateway_method" "launcher_api_method" {
  rest_api_id   = aws_api_gateway_rest_api.launcher_api.id
  resource_id   = aws_api_gateway_resource.launcher_api_resource.id
  http_method   = "GET"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "launcher_api_method-integration" {
  rest_api_id             = aws_api_gateway_rest_api.launcher_api.id
  resource_id             = aws_api_gateway_resource.launcher_api_resource.id
  http_method             = aws_api_gateway_method.launcher_api_method.http_method
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.launcher_function.invoke_arn
  integration_http_method = "POST"
}


resource "aws_api_gateway_deployment" "example_deployment_prod" {
  depends_on = [
    aws_api_gateway_method.launcher_api_method,
    aws_api_gateway_integration.launcher_api_method-integration
  ]
  rest_api_id = aws_api_gateway_rest_api.launcher_api.id
  stage_name  = "api"
}

output "prod_url" {
  value = "https://${aws_api_gateway_deployment.example_deployment_prod.rest_api_id}.execute-api.${var.region}.amazonaws.com/${aws_api_gateway_deployment.example_deployment_prod.stage_name}"
}
