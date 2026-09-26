package response

import "net/http"

const (
	CodeRequestInvalidBody       = "request.invalid_body"
	CodeRequestInvalid           = "request.invalid"
	CodeRequestInvalidID         = "request.invalid_id"
	CodeRequestInvalidQuery      = "request.invalid_query"
	CodeRequestRequired          = "request.required"
	CodeAuthUnauthorized         = "auth.unauthorized"
	CodeAuthForbidden            = "auth.forbidden"
	CodeAuthInvalidToken         = "auth.invalid_token"
	CodeAuthInvalidCredentials   = "auth.invalid_credentials"
	CodeAuthInvalidCurrentPass   = "auth.invalid_current_password"
	CodeAuthInvalidRefreshToken  = "auth.invalid_refresh_token"
	CodeAuthInvalidTwoFactorCode = "auth.invalid_two_factor_code"
	CodeAuthTwoFactorExpired     = "auth.two_factor_expired"
	CodeAuthTwoFactorNotStarted  = "auth.two_factor_not_started"
	CodeAuthLastLoginRequired    = "auth.last_login_method_required"
	CodeAuthSessionInvalid       = "auth.session_invalid"
	CodeResourceNotFound         = "resource.not_found"
	CodeResourceConflict         = "resource.conflict"
	CodeBillingPaymentRequired   = "billing.payment_required"
	CodeBillingInsufficientFunds = "billing.insufficient_funds"
	CodeBillingPricingRequired   = "billing.pricing_required"
	CodeRateLimitExceeded        = "rate_limit.exceeded"
	CodeQuotaExceeded            = "quota.exceeded"
	CodeFileInUse                = "file.in_use"
	CodeFileTooLarge             = "file.too_large"
	CodeFileNotReady             = "file.not_ready"
	CodeFileTypeBlocked          = "file.type_blocked"
	CodeUpstreamUnavailable      = "upstream.unavailable"
	CodeUpstreamRateLimited      = "upstream.rate_limited"
	CodeServiceUnavailable       = "service.unavailable"
	CodeInternal                 = "internal.error"
)

// codeMessages 只登记由响应边界直接选择的错误码。应用层错误的文案与错误码由 apperr 自身声明，
// 不在这里重复维护。
var codeMessages = map[string]string{
	CodeRequestInvalidBody:       "invalid request body",
	CodeRequestInvalid:           "invalid request",
	CodeRequestInvalidID:         "invalid id",
	CodeRequestInvalidQuery:      "invalid query parameter",
	CodeRequestRequired:          "required field missing",
	CodeAuthUnauthorized:         "unauthorized",
	CodeAuthForbidden:            "forbidden",
	CodeAuthInvalidToken:         "invalid token",
	CodeAuthInvalidCredentials:   "invalid username or password",
	CodeAuthInvalidCurrentPass:   "invalid current password",
	CodeAuthInvalidRefreshToken:  "invalid refresh token",
	CodeAuthInvalidTwoFactorCode: "invalid two factor code",
	CodeAuthTwoFactorExpired:     "two factor challenge expired",
	CodeAuthTwoFactorNotStarted:  "two factor setup not started",
	CodeAuthLastLoginRequired:    "set a password or bind another identity provider first",
	CodeAuthSessionInvalid:       "session invalid",
	CodeResourceNotFound:         "resource not found",
	CodeResourceConflict:         "resource conflict",
	CodeBillingPaymentRequired:   "payment required",
	CodeBillingInsufficientFunds: "insufficient balance",
	CodeBillingPricingRequired:   "model pricing is required",
	CodeRateLimitExceeded:        "rate limit exceeded",
	CodeQuotaExceeded:            "quota exceeded",
	CodeFileInUse:                "file is in use",
	CodeFileTooLarge:             "file too large",
	CodeFileNotReady:             "file is not ready",
	CodeFileTypeBlocked:          "file type is not allowed",
	CodeUpstreamUnavailable:      "upstream service unavailable",
	CodeUpstreamRateLimited:      "upstream rate limited",
	CodeServiceUnavailable:       "service unavailable",
	CodeInternal:                 "internal server error",

	"auth.provider_email_conflict":                  "provider email belongs to another account",
	"billing.invalid_redemption_code":               "invalid redemption code",
	"content_moderation.config_required":            "content moderation service config and policy are required when enabled",
	"content_moderation.invalid_config":             "invalid content moderation config",
	"content_moderation.probe_failed":               "content moderation probe failed",
	"conversation.message_fork_history_incomplete":  "message history is too deep or incomplete",
	"conversation.message_fork_state_invalid":       "message is still generating",
	"conversation.message_fork_target_invalid":      "only assistant messages can be forked",
	"cors.origin_forbidden":                         "origin is not allowed",
	"embedding.service_not_configured":              "embedding service is not configured",
	"embedding.service_unavailable":                 "embedding service is not available",
	"embedding.submit_failed":                       "failed to submit embedding jobs",
	"embedding.too_many_files":                      "too many files for embedding",
	"file.not_found":                                "file not found",
	"identity_provider.delete_conflict":             "deleting this identity provider would remove the only login method for some users",
	"knowledge_base.conflict":                       "knowledge base conflict",
	"knowledge_base.disabled":                       "knowledge base feature is disabled",
	"knowledge_base.file_cleanup_unavailable":       "platform file cleanup unavailable",
	"knowledge_base.internal":                       "knowledge base operation failed",
	"knowledge_base.invalid":                        "invalid knowledge base request",
	"knowledge_base.not_found":                      "knowledge base not found",
	"knowledge_base.owner_file_reference":           "user owns files referenced by builtin knowledge bases",
	"knowledge_base.platform_file_in_use":           "platform file is in use",
	"llm.empty_response":                            "model returned empty response",
	"llm.model_icon_asset_in_use":                   "model icon asset is in use",
	"llm.model_vendor_builtin":                      "built-in model vendor cannot be deleted",
	"llm.model_vendor_in_use":                       "model vendor is in use",
	"llm.remote_models_empty_confirmation_required": "remote models snapshot is empty",
	"llm.remote_models_snapshot_changed":            "remote models snapshot changed",
	"llm.upstream_model_binding_changed":            "upstream model binding changed; reload and retry",
	"llm.upstream_model_conflict":                   "model upstream source conflict",
	"media.artifact_unavailable":                    "generated media artifact is temporarily unavailable",
	"media.image_stream_unsupported":                "upstream may not support image streaming; disable image.stream for this model",
	"payment.checkout_failed":                       "create checkout failed",
	"payment.epay_gateway_invalid":                  "epay gateway url is invalid",
	"payment.provider_unavailable":                  "payment provider is unavailable",
	"settings.invalid_namespace":                    "invalid setting namespace",
	"settings.invalid_key":                          "invalid setting key",
	"settings.invalid_value":                        "invalid setting value",
	"settings.smtp_invalid":                         "invalid SMTP settings",
	"settings.billing_payment_invalid":              "invalid billing payment settings",
	"settings.embedding_invalid":                    "invalid embedding settings",
	"settings.extract_invalid":                      "invalid file extraction settings",
	"settings.model_option_policy_invalid":          "invalid model option policy settings",
	"skill.too_many_ids":                            "too many skill ids",
	"usage_statistics.invalid_billing_scope":        "invalid billing scope",
	"usage_statistics.invalid_date_range":           "invalid usage statistics date range",
	"usage_statistics.invalid_rank_by":              "invalid usage ranking field",
	"usage_statistics.invalid_section":              "invalid usage statistics section",
	"usage_statistics.subject_conflict":             "user and permission group filters are mutually exclusive",
}

<<<<<<< HEAD
var exactErrorSpecs = map[string]errorSpec{
	"unauthorized":                                               {Code: CodeAuthUnauthorized, Message: "unauthorized"},
	"forbidden":                                                  {Code: CodeAuthForbidden, Message: "forbidden"},
	"admin permission required":                                  {Code: "auth.admin_required", Message: "admin permission required"},
	"superadmin permission required":                             {Code: "auth.superadmin_required", Message: "superadmin permission required"},
	"missing authorization header":                               {Code: CodeAuthInvalidToken, Message: "authorization header is required"},
	"invalid authorization header":                               {Code: CodeAuthInvalidToken, Message: "invalid authorization header"},
	"invalid token":                                              {Code: CodeAuthInvalidToken, Message: "invalid token"},
	"invalid token type":                                         {Code: CodeAuthInvalidToken, Message: "invalid token type"},
	"session invalid":                                            {Code: CodeAuthSessionInvalid, Message: "session invalid"},
	"invalid username or password":                               {Code: CodeAuthInvalidCredentials, Message: "invalid username or password"},
	"invalid current password":                                   {Code: CodeAuthInvalidCurrentPass, Message: "invalid current password"},
	"invalid refresh token":                                      {Code: CodeAuthInvalidRefreshToken, Message: "invalid refresh token"},
	"session revoked":                                            {Code: CodeAuthSessionInvalid, Message: "session invalid"},
	"invalid two factor code":                                    {Code: CodeAuthInvalidTwoFactorCode, Message: "invalid two factor code"},
	"two factor challenge expired":                               {Code: CodeAuthTwoFactorExpired, Message: "two factor challenge expired"},
	"two factor setup expired":                                   {Code: CodeAuthTwoFactorExpired, Message: "two factor setup expired"},
	"two factor setup not started":                               {Code: CodeAuthTwoFactorNotStarted, Message: "two factor setup not started"},
	"two factor setup was not persisted":                         {Code: CodeInternal, Message: "internal server error"},
	"two factor authentication is already enabled":               {Code: "auth.two_factor_already_enabled", Message: "two factor authentication is already enabled"},
	"password reset required":                                    {Code: "auth.password_reset_required", Message: "password reset required"},
	"password reset failed":                                      {Code: "auth.password_reset_failed", Message: "password reset failed"},
	"username change required":                                   {Code: "auth.username_change_required", Message: "username change required"},
	"password length must be between 6 and 128":                  {Code: "auth.invalid_password", Message: "password length must be between 6 and 128"},
	"invalid password":                                           {Code: "auth.invalid_password", Message: "password must be at least 8 characters and not digits only"},
	"new password must be different from the bootstrap password": {Code: "auth.password_reuse_not_allowed", Message: "new password must be different from the bootstrap password"},
	"account locked":                                             {Code: CodeAuthInvalidCredentials, Message: "invalid username or password"},
	"cannot unlink the last available login method":              {Code: CodeAuthLastLoginRequired, Message: "set a password or bind another identity provider first"},
	"configure the provider callback url to the frontend callback endpoint": {Code: "auth.provider_callback_misconfigured", Message: "configure the provider callback URL to the frontend callback endpoint"},
	"email registration is disabled":                                        {Code: "auth.email_registration_disabled", Message: "email registration is disabled"},
	"email verification is disabled":                                        {Code: "auth.email_verification_disabled", Message: "email verification is disabled"},
	"email already exists":                                                  {Code: "auth.email_already_exists", Message: "email already exists"},
	"user email is invalid":                                                 {Code: "auth.invalid_email", Message: "invalid email"},
	"admin email is invalid":                                                {Code: "auth.invalid_email", Message: "invalid email"},
	"invalid email":                                                         {Code: "auth.invalid_email", Message: "invalid email"},
	"user email is not verified":                                            {Code: "auth.email_not_verified", Message: "email is not verified"},
	"admin email is not verified":                                           {Code: "auth.email_not_verified", Message: "email is not verified"},
	"current email is not verified":                                         {Code: "auth.email_not_verified", Message: "email is not verified"},
	"new email must be different":                                           {Code: "auth.email_unchanged", Message: "new email must be different"},
	"email aliases are not allowed":                                         {Code: "auth.email_alias_not_allowed", Message: "email aliases are not allowed"},
	"email domain is not allowed":                                           {Code: "auth.email_domain_not_allowed", Message: "email domain is not allowed"},
	"email bootstrap is not allowed":                                        {Code: "auth.email_bootstrap_not_allowed", Message: "email bootstrap is not allowed"},
	"turnstile is not configured":                                           {Code: "auth.turnstile_not_configured", Message: "turnstile is not configured"},
	"turnstile verification is required":                                    {Code: "auth.turnstile_required", Message: "turnstile verification is required"},
	"turnstile token is too long":                                           {Code: "auth.turnstile_invalid", Message: "turnstile token is invalid"},
	"turnstile verification failed":                                         {Code: "auth.turnstile_invalid", Message: "turnstile verification failed"},
	"third-party login is disabled":                                         {Code: "auth.provider_login_disabled", Message: "third-party login is disabled"},
	"authorization code is required":                                        {Code: "auth.authorization_code_required", Message: "authorization code is required"},
	"provider id is required":                                               {Code: "auth.provider_id_required", Message: "provider id is required"},
	"provider bind must use account binding endpoint":                       {Code: "auth.provider_bind_endpoint_required", Message: "provider bind must use account binding endpoint"},
	"provider email belongs to another account; sign in to that account or change its email before binding": {
		Code:    "auth.provider_email_conflict",
		Message: "provider email belongs to another account",
	},
	"invalid redirect uri":                                        {Code: "auth.invalid_redirect_uri", Message: "invalid redirect uri"},
	"redirect uri origin is not allowed":                          {Code: "auth.invalid_redirect_uri", Message: "redirect uri origin is not allowed"},
	"provider ids must be unique":                                 {Code: "auth.provider_order_invalid", Message: "provider ids must be unique"},
	"provider type must be oidc or oauth2":                        {Code: "auth.provider_type_invalid", Message: "provider type must be oidc or oauth2"},
	"provider name is required":                                   {Code: "auth.provider_name_required", Message: "provider name is required"},
	"provider slug is required":                                   {Code: "auth.provider_slug_required", Message: "provider slug is required"},
	"default role must be user, admin or superadmin":              {Code: "auth.provider_default_role_invalid", Message: "default role must be user, admin or superadmin"},
	"only superadmin can set superadmin default role":             {Code: "auth.provider_superadmin_default_role_protected", Message: "only superadmin can set superadmin default role"},
	"logo url must be a valid http(s) or absolute path":           {Code: "auth.provider_logo_url_invalid", Message: "logo url must be a valid http(s) or absolute path"},
	"provider registration requires provider login to be enabled": {Code: "auth.provider_registration_requires_login", Message: "provider registration requires provider login to be enabled"},
	"client id is required":                                       {Code: "auth.provider_client_id_required", Message: "client id is required"},
	"client secret is required":                                   {Code: "auth.provider_client_secret_required", Message: "client secret is required"},
	"oidc issuer url or discovery url is required":                {Code: "auth.provider_oidc_issuer_required", Message: "OIDC issuer url or discovery url is required"},
	"oauth2 auth url, token url and userinfo url are required":    {Code: "auth.provider_oauth_urls_required", Message: "OAuth2 auth url, token url and userinfo url are required"},
	"provider auth url is not configured":                         {Code: "auth.provider_auth_url_not_configured", Message: "provider auth url is not configured"},

	"invalid time zone":                       {Code: "user.invalid_time_zone", Message: "invalid time zone"},
	"invalid timezone":                        {Code: "user.invalid_time_zone", Message: "invalid time zone"},
	"invalid location":                        {Code: "user.invalid_location", Message: "invalid location"},
	"invalid avatar url":                      {Code: "user.invalid_avatar_url", Message: "invalid avatar url"},
	"invalid username":                        {Code: "user.invalid_username", Message: "invalid username"},
	"invalid display name":                    {Code: "user.invalid_display_name", Message: "invalid display name"},
	"invalid user email":                      {Code: "user.invalid_email", Message: "invalid user email"},
	"invalid user phone":                      {Code: "user.invalid_phone", Message: "invalid user phone"},
	"invalid user locale":                     {Code: "user.invalid_locale", Message: "invalid user locale"},
	"invalid user status":                     {Code: "user.invalid_status", Message: "invalid user status"},
	"invalid user role":                       {Code: "user.invalid_role", Message: "invalid user role"},
	"invalid user timezone":                   {Code: "user.invalid_time_zone", Message: "invalid time zone"},
	"username already exists":                 {Code: "user.username_already_exists", Message: "username already exists"},
	"username change already used":            {Code: "user.username_change_used", Message: "username change already used"},
	"superadmin account deletion not allowed": {Code: "user.superadmin_delete_protected", Message: "superadmin account deletion is not allowed"},
	"account deletion requires verification":  {Code: "user.account_delete_verification_required", Message: "account deletion requires verification"},
	"superadmin delete not allowed":           {Code: "user.superadmin_delete_protected", Message: "superadmin account deletion is not allowed"},
	"superadmin status change not allowed":    {Code: "user.superadmin_status_protected", Message: "superadmin status change is not allowed"},
	"superadmin password reset not allowed":   {Code: "user.superadmin_password_reset_protected", Message: "superadmin password reset is not allowed"},
	"superadmin two factor reset not allowed": {Code: "user.superadmin_two_factor_reset_protected", Message: "superadmin two factor reset is not allowed"},
	"superadmin management not allowed":       {Code: "user.superadmin_management_protected", Message: "superadmin management is not allowed"},
	"last superadmin role change not allowed": {Code: "user.last_superadmin_role_protected", Message: "last superadmin role change is not allowed"},
	"self role change not allowed":            {Code: "user.self_role_change_not_allowed", Message: "self role change is not allowed"},
	"self status change not allowed":          {Code: "user.self_status_change_not_allowed", Message: "self status change is not allowed"},
	"self delete not allowed":                 {Code: "user.self_delete_not_allowed", Message: "self delete is not allowed"},
	"empty admin user patch":                  {Code: "user.empty_patch", Message: "at least one user field is required"},

	"invalid conversation title":                              {Code: "conversation.invalid_title", Message: "invalid conversation title"},
	"conversation has no titleable content":                   {Code: "conversation.no_titleable_content", Message: "conversation has no titleable content"},
	"invalid conversation share":                              {Code: "conversation_share.invalid", Message: "invalid conversation share"},
	"conversation share schema outdated":                      {Code: "conversation_share.schema_outdated", Message: "conversation share schema is outdated"},
	"conversation share schema is outdated, rebuild database": {Code: "conversation_share.schema_outdated", Message: "conversation share schema is outdated"},
	"message feedback target invalid":                         {Code: "message.feedback_target_invalid", Message: "message feedback target invalid"},
	"invalid message feedback":                                {Code: "message.invalid_feedback", Message: "invalid message feedback"},
	"invalid message content":                                 {Code: "message.invalid_content", Message: "invalid message content"},
	"message edit target invalid":                             {Code: "message.edit_target_invalid", Message: "message edit target invalid"},
	"message edit state invalid":                              {Code: "message.edit_state_invalid", Message: "message edit state invalid"},
	"invalid message branch":                                  {Code: "message.invalid_branch", Message: "invalid message branch"},
	"message generation canceled":                             {Code: "conversation_run.canceled", Message: "message generation canceled"},
	"too many files in one message":                           {Code: "message.too_many_files", Message: "too many files in one message"},
	"too many selected tools":                                 {Code: "message.too_many_selected_tools", Message: "too many selected tools"},
	"multiple image attachment processors selected":           {Code: "message.multiple_image_processors", Message: "select only one image attachment processor"},
	"image attachment processing failed":                      {Code: "mcp.image_processing_failed", Message: "image processing tool failed"},
	"too many selected skills":                                {Code: "message.too_many_selected_skills", Message: "too many selected skills"},
	"generation stream not found":                             {Code: "conversation_run.stream_not_found", Message: "generation stream not found"},
	"image prompt is required":                                {Code: "media.image_prompt_required", Message: "image prompt is required"},
	"image generation does not accept input images":           {Code: "media.image_generation_rejects_inputs", Message: "image generation does not accept input images"},
	"image edit requires at least one input image":            {Code: "media.image_edit_input_required", Message: "image edit requires at least one input image"},
	"too many image edit input images":                        {Code: "media.image_edit_too_many_inputs", Message: "too many image edit input images"},
	"image edit input image is invalid":                       {Code: "media.image_edit_input_invalid", Message: "image edit input image is invalid"},
	"video prompt is required":                                {Code: "media.video_prompt_required", Message: "video prompt is required"},
	"video generation input is invalid":                       {Code: "media.video_input_invalid", Message: "video generation input is invalid"},
	"too many video generation input images":                  {Code: "media.video_too_many_inputs", Message: "too many video generation input images"},
	"media route protocol does not match task":                {Code: "media.route_protocol_mismatch", Message: "media route protocol does not match task"},
	"invalid media generation task":                           {Code: "media.invalid_task", Message: "invalid media generation task"},
	"invalid mcp tool attachment configuration":               {Code: "mcp.invalid_attachment_configuration", Message: "invalid MCP tool attachment configuration"},

	"file is required":                                     {Code: "file.required", Message: "file is required"},
	"invalid file stream":                                  {Code: "file.invalid_stream", Message: "invalid file stream"},
	"invalid file reference":                               {Code: "file.invalid_reference", Message: "invalid file reference"},
	"invalid file name":                                    {Code: "file.invalid_name", Message: "invalid file name"},
	"storage quota exceeded":                               {Code: CodeQuotaExceeded, Message: "storage quota exceeded"},
	"dangerous file type not allowed":                      {Code: CodeFileTypeBlocked, Message: "file type is not allowed"},
	"mime blocked":                                         {Code: CodeFileTypeBlocked, Message: "file type is not allowed"},
	"embedding unavailable":                                {Code: "file.embedding_unavailable", Message: "embedding is unavailable"},
	"embedding unavailable for this file size":             {Code: "file.embedding_unavailable", Message: "embedding is unavailable for this file size"},
	"embedding unavailable for current file capability":    {Code: "file.embedding_unavailable", Message: "embedding is unavailable for current file capability"},
	"file is in use":                                       {Code: CodeFileInUse, Message: "file is in use"},
	"file too large":                                       {Code: CodeFileTooLarge, Message: "file too large"},
	"file processing not ready":                            {Code: CodeFileNotReady, Message: "file processing is not ready"},
	"file extract not ready":                               {Code: "file.extract_not_ready", Message: "file extract is not ready"},
	"file too large for full context":                      {Code: "file.too_large_for_context", Message: "file is too large for full context"},
	"at least one of file_name or rag_opt_out is required": {Code: CodeRequestRequired, Message: "at least one of file_name or rag_opt_out is required"},

	"invalid file path":                {Code: "skill.invalid_file_path", Message: "invalid file path"},
	"skill package file is required":   {Code: "skill.package_file_required", Message: "skill package file is required"},
	"skill package file is too large":  {Code: "skill.package_file_too_large", Message: "skill package file is too large"},
	"skill package file is unreadable": {Code: "skill.package_file_unreadable", Message: "skill package file is unreadable"},
	"skill trigger already exists":     {Code: "skill.trigger_already_exists", Message: "skill trigger already exists"},

	"invalid billing plan":                           {Code: "billing.invalid_plan", Message: "invalid billing plan"},
	"billing plan not found":                         {Code: "billing.plan_not_found", Message: "billing plan not found"},
	"invalid permission group":                       {Code: "billing.invalid_permission_group", Message: "invalid permission group"},
	"permission group not found":                     {Code: "admin.permission_group_not_found", Message: "permission group not found"},
	"invalid permission group name":                  {Code: "admin.invalid_permission_group_name", Message: "invalid permission group name"},
	"invalid permission group rate multiplier":       {Code: "admin.invalid_permission_group_rate_multiplier", Message: "invalid permission group rate multiplier"},
	"invalid permission group models":                {Code: "admin.invalid_permission_group_models", Message: "invalid permission group models"},
	"invalid permission group users":                 {Code: "admin.invalid_permission_group_users", Message: "invalid permission group users"},
	"default permission group delete not allowed":    {Code: "admin.default_permission_group_delete_not_allowed", Message: "default permission group cannot be deleted"},
	"default permission group users are implicit":    {Code: "admin.default_permission_group_users_implicit", Message: "default permission group users are implicit"},
	"permission group is referenced by billing plan": {Code: "admin.permission_group_referenced_by_plan", Message: "permission group is referenced by a billing plan"},

	"model route not configured":                  {Code: "llm.model_route_not_configured", Message: "model route is not configured"},
	"model access denied by group policy":         {Code: "llm.model_access_denied", Message: "you do not have access to this model"},
	"model returned empty response":               {Code: "llm.empty_response", Message: "model returned empty response"},
	"upstream returned empty response":            {Code: "llm.empty_response", Message: "model returned empty response"},
	"remote models unavailable":                   {Code: "llm.remote_models_unavailable", Message: "remote models unavailable"},
	"no active api key":                           {Code: "llm.no_active_api_key", Message: "no active api key"},
	"invalid adapter":                             {Code: "llm.invalid_adapter", Message: "invalid adapter"},
	"invalid compatible":                          {Code: "llm.invalid_compatible", Message: "invalid compatible"},
	"invalid json config":                         {Code: "config.invalid_json", Message: "invalid json config"},
	"invalid headers config":                      {Code: "llm.invalid_headers_config", Message: "invalid headers json config"},
	"invalid headers json config":                 {Code: "llm.invalid_headers_config", Message: "invalid headers json config"},
	"invalid api keys config":                     {Code: "llm.invalid_api_keys_config", Message: "invalid api keys config"},
	"invalid protocol defaults config":            {Code: "llm.invalid_protocol_defaults_config", Message: "invalid protocol defaults config"},
	"invalid kinds":                               {Code: "llm.invalid_kinds", Message: "invalid model kinds"},
	"invalid route protocol combination":          {Code: "llm.invalid_route_protocol_combination", Message: "invalid route protocol combination"},
	"invalid platform model name":                 {Code: "llm.invalid_platform_model_name", Message: "invalid platform model name"},
	"system prompt too long":                      {Code: "llm.system_prompt_too_long", Message: "system prompt too long"},
	"platform model name is required":             {Code: "llm.platform_model_name_required", Message: "platform model name is required"},
	"protocol required":                           {Code: "llm.protocol_required", Message: "protocol is required"},
	"platform model name already exists":          {Code: "llm.platform_model_name_exists", Message: "platform model name already exists"},
	"target model already bound on this upstream": {Code: "llm.route_conflict", Message: "target model is already bound on this upstream"},
	"all routes unavailable":                      {Code: "llm.routes_unavailable", Message: "all model routes are unavailable"},
	"upstream source unavailable":                 {Code: "llm.upstream_source_unavailable", Message: "upstream source unavailable"},
	"route not found":                             {Code: "route.not_found", Message: "route not found"},
	"api_keys is required":                        {Code: "llm.api_keys_required", Message: "api_keys is required"},
	"invalid model icon":                          {Code: "llm.model_icon_invalid", Message: "invalid model icon"},
	"invalid model icon file":                     {Code: "llm.model_icon_file_invalid", Message: "invalid model icon file"},
	"model icon file too large":                   {Code: "llm.model_icon_file_too_large", Message: "model icon file too large"},
	"model icon asset not found":                  {Code: "llm.model_icon_asset_not_found", Message: "model icon asset not found"},
	"model icon asset is in use":                  {Code: "llm.model_icon_asset_in_use", Message: "model icon asset is in use"},
	"built-in model vendor cannot be deleted":     {Code: "llm.model_vendor_builtin", Message: "built-in model vendor cannot be deleted"},
	"model vendor is in use":                      {Code: "llm.model_vendor_in_use", Message: "model vendor is in use"},
	"circuit breaker is disabled":                 {Code: "llm.circuit_breaker_disabled", Message: "circuit breaker is disabled"},

	"usage balance is insufficient":                                {Code: CodeBillingInsufficientFunds, Message: "insufficient balance"},
	"usage concurrency limit exceeded":                             {Code: "billing.concurrency_limit_exceeded", Message: "too many concurrent paid requests"},
	"usage reservation already exists":                             {Code: "billing.reservation_conflict", Message: "usage request already exists"},
	"model pricing is required":                                    {Code: CodeBillingPricingRequired, Message: "model pricing is required"},
	"period usage credit exceeded":                                 {Code: "billing.period_credit_exceeded", Message: "period usage credit exceeded"},
	"invalid subscription tier":                                    {Code: "billing.invalid_subscription_tier", Message: "invalid subscription tier"},
	"subscription expiry required":                                 {Code: "billing.subscription_expiry_required", Message: "subscription expiry required"},
	"invalid subscription expiry":                                  {Code: "billing.invalid_subscription_expiry", Message: "invalid subscription expiry"},
	"subscription entitlement is active":                           {Code: "billing.subscription_entitlement_active", Message: "subscription entitlement is active"},
	"invalid model pricing":                                        {Code: "billing.invalid_model_pricing", Message: "invalid model pricing"},
	"invalid daily usage date range":                               {Code: "billing.invalid_daily_usage_date_range", Message: "invalid daily usage date range"},
	"invalid daily usage days":                                     {Code: "billing.invalid_daily_usage_days", Message: "invalid daily usage days"},
	"redemption code hash secret unavailable":                      {Code: "billing.redemption_secret_unavailable", Message: "redemption code service is unavailable"},
	"invalid redemption code":                                      {Code: "billing.invalid_redemption_code", Message: "invalid redemption code"},
	"redemption code already exists":                               {Code: "billing.redemption_code_conflict", Message: "redemption code already exists"},
	"redemption code is unavailable":                               {Code: "billing.redemption_code_unavailable", Message: "redemption code is unavailable"},
	"redemption code plaintext unavailable":                        {Code: "billing.redemption_code_plaintext_unavailable", Message: "redemption code plaintext unavailable"},
	"redemption code exhausted":                                    {Code: "billing.redemption_code_exhausted", Message: "redemption code exhausted"},
	"redemption user limit exceeded":                               {Code: "billing.redemption_user_limit_exceeded", Message: "redemption user limit exceeded"},
	"payment is required":                                          {Code: CodeBillingPaymentRequired, Message: "payment is required"},
	"payment provider is unavailable":                              {Code: "payment.provider_unavailable", Message: "payment provider is unavailable"},
	"epay gateway url is invalid":                                  {Code: "payment.epay_gateway_invalid", Message: "epay gateway url is invalid"},
	"create checkout failed":                                       {Code: "payment.checkout_failed", Message: "create checkout failed"},
	"provider mismatch":                                            {Code: "payment.notification_mismatch", Message: "payment notification does not match the order"},
	"checkout id mismatch":                                         {Code: "payment.notification_mismatch", Message: "payment notification does not match the order"},
	"amount mismatch":                                              {Code: "payment.notification_mismatch", Message: "payment notification does not match the order"},
	"currency mismatch":                                            {Code: "payment.notification_mismatch", Message: "payment notification does not match the order"},
	"merchant mismatch":                                            {Code: "payment.notification_mismatch", Message: "payment notification does not match the order"},
	"epay payment type is not supported":                           {Code: "payment.epay_type_unsupported", Message: "epay payment type is not supported"},
	"payment return url is invalid":                                {Code: "payment.return_url_invalid", Message: "payment return url is invalid"},
	"payment return url must use the configured public web origin": {Code: "payment.return_url_cross_origin", Message: "payment return url must use the configured public web origin"},
	"stripe webhook is not configured":                             {Code: "payment.webhook_not_configured", Message: "stripe webhook is not configured"},
	"read webhook body failed":                                     {Code: "payment.invalid_webhook_body", Message: "invalid webhook body"},
	"webhook body too large":                                       {Code: "payment.webhook_body_too_large", Message: "webhook body too large"},
	"invalid stripe signature":                                     {Code: "payment.invalid_signature", Message: "invalid stripe signature"},
	"invalid stripe event":                                         {Code: "payment.invalid_event", Message: "invalid stripe event"},
	"missing order_no":                                             {Code: "payment.order_no_required", Message: "order_no is required"},

	"invalid namespace":                     {Code: "settings.invalid_namespace", Message: "invalid namespace"},
	"invalid setting key":                   {Code: "settings.invalid_key", Message: "invalid setting key"},
	"setting not found":                     {Code: "settings.not_found", Message: "setting not found"},
	"settings service unavailable":          {Code: "settings.service_unavailable", Message: "settings service unavailable"},
	"invalid id":                            {Code: CodeRequestInvalidID, Message: "invalid id"},
	"embedding service not available":       {Code: "embedding.service_unavailable", Message: "embedding service is not available"},
	"embedding service not configured":      {Code: "embedding.service_not_configured", Message: "embedding service is not configured"},
	"tika runtime service unavailable":      {Code: "runtime.tika_unavailable", Message: "tika runtime service unavailable"},
	"docling runtime service unavailable":   {Code: "runtime.docling_unavailable", Message: "docling runtime service unavailable"},
	"tesseract runtime service unavailable": {Code: "runtime.tesseract_unavailable", Message: "tesseract runtime service unavailable"},
	"rapidocr runtime service unavailable":  {Code: "runtime.rapidocr_unavailable", Message: "rapidocr runtime service unavailable"},
	"mineru runtime service unavailable":    {Code: "runtime.mineru_unavailable", Message: "mineru runtime service unavailable"},

	"memory_key is required":          {Code: "memory.key_required", Message: "memory_key is required"},
	"invalid mcp server id":           {Code: "mcp.server.invalid_id", Message: "invalid mcp server id"},
	"invalid mcp tool id":             {Code: "mcp.tool.invalid_id", Message: "invalid mcp tool id"},
	"invalid mcp server name":         {Code: "mcp.invalid_server_name", Message: "invalid mcp server name"},
	"invalid mcp server base url":     {Code: "mcp.invalid_server_base_url", Message: "invalid mcp server base url"},
	"invalid mcp server status":       {Code: "mcp.invalid_server_status", Message: "invalid mcp server status"},
	"invalid mcp server headers json": {Code: "mcp.invalid_server_headers", Message: "invalid mcp server headers json"},
	"invalid mcp tool status":         {Code: "mcp.invalid_tool_status", Message: "invalid mcp tool status"},
	"invalid mcp tool display name":   {Code: "mcp.invalid_tool_name", Message: "invalid mcp tool display name"},
	"invalid mcp tool description":    {Code: "mcp.invalid_tool_description", Message: "invalid mcp tool description"},
	"invalid mcp tool selection":      {Code: "mcp.invalid_tool_selection", Message: "invalid mcp tool selection"},
	"mcp client unavailable":          {Code: "mcp.client_unavailable", Message: "mcp client unavailable"},

	"rate limit exceeded":              {Code: CodeRateLimitExceeded, Message: "rate limit exceeded"},
	"too many refresh attempts":        {Code: "rate_limit.refresh_exceeded", Message: "too many refresh attempts"},
	"too many authentication attempts": {Code: "rate_limit.authentication_exceeded", Message: "too many authentication attempts"},

	"content moderation event not found":                                     {Code: "content_moderation.event_not_found", Message: "content moderation event not found"},
	"content moderation service config and policy are required when enabled": {Code: "content_moderation.config_required", Message: "content moderation service config and policy are required when enabled"},
	"invalid content moderation config":                                      {Code: "content_moderation.invalid_config", Message: "invalid content moderation config"},
	"invalid content moderation base url":                                    {Code: "content_moderation.invalid_config", Message: "invalid content moderation base url"},
	"invalid content moderation model":                                       {Code: "content_moderation.invalid_config", Message: "invalid content moderation model"},
	"content moderation probe failed":                                        {Code: "content_moderation.probe_failed", Message: "content moderation probe failed"},
	"content blocked by moderation":                                          {Code: "content_moderation.blocked", Message: "content blocked by moderation"},

	"deleting this identity provider would remove the only login method for some users": {Code: "identity_provider.delete_conflict", Message: "deleting this identity provider would remove the only login method for some users"},
=======
func canonicalMessage(code string) (string, bool) {
	message, ok := codeMessages[code]
	return message, ok
>>>>>>> upstream/dev
}

func defaultDescription(status int) Description {
	switch status {
	case http.StatusUnauthorized:
		return Description{Status: status, Code: CodeAuthUnauthorized, Message: codeMessages[CodeAuthUnauthorized]}
	case http.StatusForbidden:
		return Description{Status: status, Code: CodeAuthForbidden, Message: codeMessages[CodeAuthForbidden]}
	case http.StatusNotFound:
		return Description{Status: status, Code: CodeResourceNotFound, Message: codeMessages[CodeResourceNotFound]}
	case http.StatusConflict:
		return Description{Status: status, Code: CodeResourceConflict, Message: codeMessages[CodeResourceConflict]}
	case http.StatusPaymentRequired:
		return Description{Status: status, Code: CodeBillingPaymentRequired, Message: codeMessages[CodeBillingPaymentRequired]}
	case http.StatusTooManyRequests:
		return Description{Status: status, Code: CodeRateLimitExceeded, Message: codeMessages[CodeRateLimitExceeded]}
	case http.StatusBadGateway:
		return Description{Status: status, Code: CodeUpstreamUnavailable, Message: codeMessages[CodeUpstreamUnavailable]}
	case http.StatusServiceUnavailable:
		return Description{Status: status, Code: CodeServiceUnavailable, Message: codeMessages[CodeServiceUnavailable]}
	default:
		if status >= http.StatusInternalServerError {
			return Description{Status: status, Code: CodeInternal, Message: codeMessages[CodeInternal]}
		}
		return Description{Status: status, Code: CodeRequestInvalid, Message: codeMessages[CodeRequestInvalid]}
	}
}
