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
