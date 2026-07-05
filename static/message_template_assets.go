package static

import _ "embed"

//go:embed message_templates/register_email_code.html
var RegisterEmailCodeTemplateHTML string
