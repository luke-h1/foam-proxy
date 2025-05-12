declare global {
  namespace NodeJS {
    interface ProcessEnv {
      TWITCH_CLIENT_ID: string;
      TWITCH_CLIENT_SECRET: string;
      API_KEY: string;
    }
  }
}

export {};
