FROM ghcr.io/absmach/magistrala/ui-mg@sha256:18aaf940b8da15b1cbfc1efdd726684e78adb00e8896cb248abf42541b8053eb
USER root
COPY remembered-session.cjs patch-ui.cjs /app/remembered-login/
RUN node /app/remembered-login/patch-ui.cjs \
    && mkdir -p /var/lib/magistrala-ui/sessions \
    && chown -R nextjs /var/lib/magistrala-ui
COPY patch-bootstrap.cjs /app/bootstrap-compat/patch-bootstrap.cjs
RUN node /app/bootstrap-compat/patch-bootstrap.cjs /app/apps/community/.next/server/chunks
USER nextjs
