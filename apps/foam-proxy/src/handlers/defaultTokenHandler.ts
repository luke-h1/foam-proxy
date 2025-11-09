import * as Sentry from '@sentry/serverless';
import twitchService from '@lambda/services/twitchService';

export default async function defaultTokenHandler() {
  return Sentry.startSpan(
    {
      name: 'defaultTokenHandler',
      op: 'function.defaultToken',
    },
    async () => {
      const result = await twitchService.defaultToken();
      return JSON.stringify(result, null, 2);
    },
  );
}
