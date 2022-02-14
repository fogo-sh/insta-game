resource "aws_iam_role" "launcher_api_role" {
  name               = "terraform_function_role"
  assume_role_policy = data.aws_iam_policy_document.launcher_api_policy_doc.json
}


data "aws_iam_policy_document" "launcher_api_policy_doc" {
  statement {
    actions = ["sts:AssumeRole"]
    effect  = "Allow"
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}


resource "aws_iam_role_policy_attachment" "launcher_api_iam_role_policy_attachment" {
  role       = aws_iam_role.launcher_api_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}
