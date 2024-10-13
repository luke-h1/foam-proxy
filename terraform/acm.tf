# required to add certificates + DNS for the API gateway rather than let cloudflare handle the routing


# resource "aws_acm_certificate" "root_domain" {
#   provider          = aws.us-east-1
#   domain_name       = "foam.${var.root_domain}"
#   validation_method = "DNS"
#   lifecycle {
#     create_before_destroy = true

#     # only set to false because we might need to do a full teardown
#     prevent_destroy = false
#   }
# }

data "aws_route53_zone" "domain" {
  private_zone = false
  zone_id      = var.zone_id
}

resource "aws_acm_certificate" "cert" {
  private_key       = var.private_key
  certificate_body  = var.certificate_body
  certificate_chain = var.certificate_chain
  tags = {
    Name    = "foam certificate for ${var.env}"
    stage   = var.env
    service = "foam"
  }
}
