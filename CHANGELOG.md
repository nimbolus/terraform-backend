# Changelog

## [main](https://github.com/nimbolus/terraform-backend/compare/v0.2.4...main) (2026-11-04)

### Fixes

* use route template as path label of request metrics ([0cbdcd9](https://github.com/nimbolus/terraform-backend/commit/0cbdcd930a037105fd4060a8a49a2374d44b49c1))
* set timeouts on the http servers ([96b37fa](https://github.com/nimbolus/terraform-backend/commit/96b37fa334e0be843052c4f72364d55e1c91a6ed))
* limit request body size and read it only after authentication ([000041c](https://github.com/nimbolus/terraform-backend/commit/000041c9fb6b4710a36441a6360ea778cfb5212e))

### [v0.2.4](https://github.com/nimbolus/terraform-backend/compare/v0.2.3...v0.2.4) (2026-10-28)

#### Fixes

* honor AUTH_JWT_OIDC_ISSUER_URL (#45) ([bfe2f44](https://github.com/nimbolus/terraform-backend/commit/bfe2f447658cd38d1f9337aab3f35951e5daac03)), closes [#45](https://github.com/nimbolus/terraform-backend/issues/45)

### [v0.2.3](https://github.com/nimbolus/terraform-backend/compare/v0.2.2...v0.2.3) (2026-08-22)

#### Fixes

* jwt authenticator initialization ([e34f79c](https://github.com/nimbolus/terraform-backend/commit/e34f79c590b4b7a0fba65db7b4a52953c5733729))
* jwt test names ([8494e50](https://github.com/nimbolus/terraform-backend/commit/8494e50876e93313c827c4ae6e667e1da4daf15f))

### [v0.2.2](https://github.com/nimbolus/terraform-backend/compare/v0.2.1...v0.2.2) (2026-07-26)

#### Fixes

* verify lock id on server side during force-unlock if possible ([cd29037](https://github.com/nimbolus/terraform-backend/commit/cd290373a33e973b9604ffe8a1218e27bf856a0d))

### [v0.2.1](https://github.com/nimbolus/terraform-backend/compare/v0.2.0...v0.2.1) (2026-01-25)

#### Fixes

* vault token renewal and reauthentication ([1d40d97](https://github.com/nimbolus/terraform-backend/commit/1d40d97e7bf52706f6ca12f1a02630e889e05b66))

## [v0.2.0](https://github.com/nimbolus/terraform-backend/compare/v0.1.3...v0.2.0) (2026-01-16)

### Features

* add tests for auth ([23435a7](https://github.com/nimbolus/terraform-backend/commit/23435a760cb597a209b6778a6800de819739b46e))
* add tests for kms ([381a2e4](https://github.com/nimbolus/terraform-backend/commit/381a2e416539eebbaa987076e111bdb4255ee600))

### Fixes

* set audience in jwt auth test ([b060038](https://github.com/nimbolus/terraform-backend/commit/b0600380ebdab70f0a8a8187a9a783064ba1ec46))
* struct name in jwt auth ([09d47ea](https://github.com/nimbolus/terraform-backend/commit/09d47eafed8fbdd3b410aef59dcc0a2bbb6aac83))
* bump go versions to 1.24 ([4726115](https://github.com/nimbolus/terraform-backend/commit/47261159ef556f87567f122ed7adc1c213820281))
* add audience validation for jwt tokens ([9e74944](https://github.com/nimbolus/terraform-backend/commit/9e749448e1c31162740cefd42567db1c7e8bd98c))

### [v0.1.3](https://github.com/nimbolus/terraform-backend/compare/v0.1.2...v0.1.3) (2025-02-24)

#### Features

* allow to specify secrets using _FILE env vars ([b2d9c5a](https://github.com/nimbolus/terraform-backend/commit/b2d9c5a2f1f17911dd1d9b1faa98f49f777cbae2))

### [v0.1.2](https://github.com/nimbolus/terraform-backend/compare/v0.1.1...v0.1.2) (2024-05-07)

#### Features

* add force-unlock feature flag ([7bf26c9](https://github.com/nimbolus/terraform-backend/commit/7bf26c9bc5e7401e107008ae29331dd772d15906))
* add test for force-unlock ([4226109](https://github.com/nimbolus/terraform-backend/commit/42261098b40c7d54e5c38ba6c9cd32076346d21f))

#### Fixes

* return existing lockinfo when already locked ([378ab44](https://github.com/nimbolus/terraform-backend/commit/378ab448dbcf56d05d8824e2ae2440ec1d879563))

### [v0.1.1](https://github.com/nimbolus/terraform-backend/compare/v0.1.0...v0.1.1) (2023-03-29)

## v0.1.0 (2023-03-28)
