declare global {
  namespace NodeJS {
    interface ProcessEnv {
      API_KEY: string;
      NEW_RELIC_APP_NAME?: string;
      NEW_RELIC_ENABLED?: string;
      NEW_RELIC_LICENSE_KEY?: string;
      NEW_RELIC_LOG_LEVEL?: string;
      SENTRY_ENVIRONMENT?: string;
      SENTRY_PROXY_DSN?: string;
      SENTRY_RELEASE?: string;
      TWITCH_CLIENT_ID: string;
      TWITCH_CLIENT_SECRET: string;
    }
  }
}

export {};
