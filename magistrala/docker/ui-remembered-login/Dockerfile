FROM ghcr.io/absmach/magistrala/ui-mg@sha256:18aaf940b8da15b1cbfc1efdd726684e78adb00e8896cb248abf42541b8053eb
USER root
COPY remembered-session.cjs patch-ui.cjs /app/remembered-login/
RUN node /app/remembered-login/patch-ui.cjs \
    && node --check /app/apps/community/.next/server/chunks/41170.js \
    && node --check /app/apps/community/.next/server/chunks/59139.js \
    && mkdir -p /var/lib/magistrala-ui/sessions \
    && chown -R nextjs /var/lib/magistrala-ui
USER nextjs
