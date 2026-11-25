typedef void *viva_auth_context;
int viva_systemauth_method(void);
viva_auth_context viva_systemauth_new(void);
void viva_systemauth_cancel(viva_auth_context ctx);
void viva_systemauth_free(viva_auth_context ctx);
int viva_systemauth_verify(viva_auth_context ctx, const char *reason);
