# syntax=docker/dockerfile:1
FROM node:26-alpine AS base

WORKDIR /app

ENV NEXT_PUBLIC_BASE_PATH=/frontend

# pnpm-workspace.yaml carries the overrides / ignoredOptionalDependencies that the lockfile was
# resolved with; without it --frozen-lockfile fails with ERR_PNPM_LOCKFILE_CONFIG_MISMATCH.
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
# Installed with npm, not yarn: yarn 1.x refuses to run once package.json declares
# "packageManager": "pnpm@...". The version here tracks that field.
RUN npm install --global pnpm@11.0.9 && pnpm i --frozen-lockfile --ignore-scripts

COPY . .
RUN pnpm build

FROM node:26-alpine AS production
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_PUBLIC_BASE_PATH=/frontend

# Copy the built application from the previous stage
COPY --from=base /app/.env.production ./.env.production
COPY --from=base /app/.next/standalone ./
COPY --from=base /app/.next/static ./.next/static

# Expose the port the app runs on
EXPOSE 3000

# Set environment variable to disable telemetry
ENV NEXT_TELEMETRY_DISABLED=1

# Start the Next.js application
USER node
CMD ["node", "server.js"]
