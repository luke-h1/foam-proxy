import twitchService from '@lambda/services/twitchService';

export default async function defaultTokenHandler() {
  const result = await twitchService.defaultToken();
  return JSON.stringify(result, null, 2);
}
