# syntax=docker/dockerfile:1
FROM node:20-alpine AS base

WORKDIR /app

ENV NEXT_PUBLIC_BASE_PATH /frontend

COPY package.json yarn.lock* package-lock.json* pnpm-lock.yaml* ./
RUN \
  if [ -f yarn.lock ]; then yarn --frozen-lockfile; \
  elif [ -f package-lock.json ]; then npm ci; \
  elif [ -f pnpm-lock.yaml ]; then yarn global add pnpm && pnpm i --frozen-lockfile; \
  else echo "Lockfile not found." && exit 1; \
  fi

COPY . .
RUN yarn build

FROM node:20-alpine AS production
WORKDIR /app
ENV NODE_ENV production
ENV NEXT_PUBLIC_BASE_PATH /frontend

# Copy the built application from the previous stage
COPY --from=base /app/.next ./.next
COPY --from=base /app/node_modules ./node_modules
COPY --from=base /app/package.json ./package.json
COPY --from=base /app/next.config.mjs ./next.config.mjs
COPY --from=base /app/.env.production ./.env.production

# Expose the port the app runs on
EXPOSE 3000

# Set environment variable to disable telemetry
ENV NEXT_TELEMETRY_DISABLED 1

# Start the Next.js application
CMD ["npm", "start"]
