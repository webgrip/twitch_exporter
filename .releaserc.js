const noteKeywords = ['BREAKING CHANGE', 'BREAKING CHANGES', 'BREAKING'];

const branches = [
  'main',
  {
    name: 'release/*',
    prerelease: 'rc',
  },
];

const commitAnalyzerConfig = [
  '@semantic-release/commit-analyzer',
  {
    preset: 'conventionalcommits',
    releaseRules: [
      { type: 'feature', release: 'minor' },
      { type: 'bugfix', release: 'patch' },
      { type: 'hotfix', release: 'patch' },
    ],
    parserOpts: { noteKeywords },
  },
];

const releaseNotesGeneratorConfig = [
  '@semantic-release/release-notes-generator',
  {
    preset: 'conventionalcommits',
    presetConfig: {
      types: [
        { type: 'feat', section: 'Added' },
        { type: 'feature', section: 'Added' },
        { type: 'fix', section: 'Fixed' },
        { type: 'bugfix', section: 'Fixed' },
        { type: 'hotfix', section: 'Fixed' },
        { type: 'perf', section: 'Performance' },
        { type: 'refactor', section: 'Changed' },
        { type: 'chore', section: 'Internal', hidden: true },
        { type: 'docs', section: 'Docs', hidden: false },
        { type: 'test', section: 'Tests', hidden: false },
      ],
    },
    parserOpts: { noteKeywords },
  },
];


// Keep this commented out as you wanted
const npmConfig = [
  '@semantic-release/npm',
  {
    npmPublish: false,
  },
];

const changelogConfig = [
  '@semantic-release/changelog',
  {
    changelogFile: 'CHANGELOG.md',
  },
];

const gitConfig = [
  '@semantic-release/git',
  {
    assets: ['CHANGELOG.md', 'package.json', 'package-lock.json'],
    message: 'chore(release): ${nextRelease.version}\n\n${nextRelease.notes}',
  },
];

const execConfig = [
  '@semantic-release/exec',
  {
    successCmd: 'echo "version=${nextRelease.version}" >> $GITHUB_OUTPUT',
  },
];

const branchName =
  process.env.GITHUB_REF_NAME ||
  process.env.BRANCH_NAME ||
  '';
const isMainBranch = branchName === 'main';

const plugins = [
  commitAnalyzerConfig,
  releaseNotesGeneratorConfig,
  npmConfig,
  ...(isMainBranch ? [changelogConfig, gitConfig] : []),
  execConfig,
  '@semantic-release/github',
];

module.exports = {
  branches,
  tagFormat: '${version}',
  plugins,
};
