# syntax=docker/dockerfile:1
FROM node:25-alpine AS base

WORKDIR /app

ENV NEXT_PUBLIC_BASE_PATH /ehr

COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:25-alpine AS production
WORKDIR /app
ENV NODE_ENV production
ENV NEXT_PUBLIC_BASE_PATH /ehr

# Copy the built application from the previous stage
COPY --from=base /app/.env.production ./.env.production
COPY --from=base /app/.next/standalone ./
COPY --from=base /app/.next/static ./.next/static
COPY --from=base /app/public ./public

# Expose the port the app runs on
EXPOSE 3000

# Set environment variable to disable telemetry
ENV NEXT_TELEMETRY_DISABLED 1

# Start the Next.js application
CMD ["node", "server.js"]
