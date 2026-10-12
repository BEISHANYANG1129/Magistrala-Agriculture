FROM golang:1.24.13-alpine AS agent
WORKDIR /src
COPY llm/agentv2/go.mod llm/agentv2/go.sum ./
RUN go mod download
COPY llm/agentv2/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /agri-agent ./cmd/agent

FROM node:22-alpine AS ui
WORKDIR /app/magistrala-ui
COPY magistrala-ui/package.json magistrala-ui/package-lock.json ./
RUN npm ci
COPY magistrala-ui/ ./
RUN npm run build

FROM node:22-alpine
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=ui /app/magistrala-ui/dist ./magistrala-ui/dist
COPY magistrala-ui/server/ ./magistrala-ui/server/
COPY scripts/serve.mjs ./scripts/serve.mjs
COPY llm/agentv2/examples/ ./llm/agentv2/examples/
COPY --from=agent /agri-agent /usr/local/bin/agri-agent
RUN mkdir -p /state/sessions /state/agent && chown -R node:node /app /state
USER node
ENV AGRI_HOST=0.0.0.0 AGRI_PORT=5173 AGRI_AGENT_BINARY=/usr/local/bin/agri-agent AGRI_SESSION_DIR=/state/sessions AGRI_AGENT_STATE_DIR=/state/agent
EXPOSE 5173
CMD ["node","scripts/serve.mjs"]

