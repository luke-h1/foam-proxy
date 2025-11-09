declare global {
  namespace NodeJS {
    interface ProcessEnv {
      API_KEY: string;
      ENVIRONMENT?: string;
      SENTRY_AUTHORIZER_DSN?: string;
      SENTRY_ENVIRONMENT?: string;
      SENTRY_RELEASE?: string;
    }
  }
}

export {};
