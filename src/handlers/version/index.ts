const versionHandler = () => {
  return JSON.stringify(
    {
      version: '0.0.0-replace-me',
      deployedBy: process.env.DEPLOYED_BY,
      deployedAt: process.env.DEPLOYED_AT,
      gitSha: process.env.GIT_SHA ?? 'unknown',
    },
    null,
    2,
  );
};
