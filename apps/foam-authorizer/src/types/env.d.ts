declare global {
  namespace NodeJS {
    interface ProcessEnv {
      API_KEY: string;
      ENVIRONMENT?: string;
      NEW_RELIC_APP_NAME?: string;
      NEW_RELIC_ENABLED?: string;
      NEW_RELIC_LICENSE_KEY?: string;
      NEW_RELIC_LOG_LEVEL?: string;
      SENTRY_AUTHORIZER_DSN?: string;
      SENTRY_ENVIRONMENT?: string;
      SENTRY_RELEASE?: string;
    }
  }
}

export {};
