import axios from 'axios';

const twitchService = {
  defaultToken: async () => {
    const { data } = await axios.post(
      'https://id.twitch.tv/oauth2/token',
      null,
      {
        params: {
          client_id: process.env.TWITCH_CLIENT_ID,
          client_secret: process.env.TWITCH_CLIENT_SECRET,
          grant_type: 'client_credentials',
        },
        headers: {
          'Content-Type': 'x-www-form-urlencoded',
        },
      },
    );
    // eslint-disable-next-line @typescript-eslint/no-unsafe-return
    return data;
  },
} as const;

export default twitchService;
