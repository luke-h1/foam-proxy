/* eslint-disable no-console */
/* eslint-disable @typescript-eslint/restrict-template-expressions */
import axios from 'axios';
import * as Sentry from '@sentry/serverless';

export default async function tokenHandler() {
  return Sentry.startSpan(
    {
      name: 'tokenHandler',
      op: 'function.token',
    },
    async () => {
      try {
        const { data } = await axios.post(
          `https://id.twitch.tv/oauth2/token?client_id=${process.env.TWITCH_CLIENT_ID}&client_secret=${process.env.TWITCH_CLIENT_SECRET}&grant_type=client_credentials`,
          null,
          {},
        );

        return JSON.stringify({
          data,
          error: null,
        });
      } catch (error) {
        console.error(`tokenHandler error: ${error}`);
        Sentry.captureException(error);
        return JSON.stringify({
          data: null,
          error,
        });
      }
    },
  );
}
