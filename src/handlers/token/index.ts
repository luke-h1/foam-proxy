import axios from 'axios';

export default async function tokenHandler() {
  const { data } = await axios.post('https://id.twitch.tv/oauth2/token', null, {
    params: {
      client_id: process.env.EXPO_PUBLIC_TWITCH_CLIENT_ID,
      client_secret: process.env.EXPO_PUBLIC_TWITCH_CLIENT_SECRET,
      grant_type: 'client_credentials',
    },
    headers: {
      'Content-Type': 'x-www-form-urlencoded',
    },
  });
  return JSON.stringify({
    data,
  });
}
