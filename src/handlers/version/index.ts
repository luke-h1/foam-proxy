const versionHandler = () => {
  return JSON.stringify(
    {
      version: '0.0.0-replace-me',
      deployedBy: process.env.DEPLOYED_BY ?? 'unknown',
      deployedAt: process.env.DEPLOYED_AT ?? 'unknown',
      gitSha: process.env.GIT_SHA ?? 'unknown',
    },
    null,
    2,
  );
};
