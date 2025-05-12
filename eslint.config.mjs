import globals from 'globals';
import tseslint from 'typescript-eslint';
import importPlugin from 'eslint-plugin-import';
import jestPlugin from 'eslint-plugin-jest';

/** @type {import('eslint').Linter.Config[]} */
export default [
  ...tseslint.configs.recommendedTypeChecked,
  importPlugin.flatConfigs.recommended,
  {
    files: ['apps/**/*.{js,mjs,cjs,ts,jsx,tsx}', 'scripts/**/*.{js,mjs,cjs}'],
    ignores: ['eslint.config.mjs'],
  },
  {
    files: ['scripts/**/*.js'],
    languageOptions: {
      sourceType: 'module',
      parserOptions: {
        ecmaVersion: 'latest',
      },
      globals: {
        ...globals.node,
      },
    },
    rules: {
      '@typescript-eslint/no-var-requires': 'off',
      'no-undef': 'error',
    },
  },
  {
    files: ['**/*.test.ts', '**/*.test.tsx', '**/*.spec.ts', '**/*.spec.tsx'],
    plugins: {
      jest: jestPlugin,
    },
    rules: {
      ...jestPlugin.configs.recommended.rules,
      'jest/no-disabled-tests': 'warn',
      'jest/no-focused-tests': 'error',
      'jest/no-identical-title': 'error',
      'jest/prefer-to-have-length': 'warn',
      'jest/valid-expect': 'error',
    },
  },
  {
    languageOptions: {
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
      globals: {
        ...globals.node,
        ...globals.jest,
      },
    },
  },
  {
    settings: {
      'import/resolver': {
        typescript: {
          project: ['./tsconfig.json'],
          alwaysTryTypes: true,
          extensions: ['.ts', '.tsx', '.js', '.jsx'],
        },
        node: {
          extensions: ['.js', '.jsx', '.ts', '.tsx'],
        },
        alias: {
          map: [
            ['@lambda', './apps/foam-lambda/src'],
            ['@lambda-test', './apps/foam-lambda/src/test'],
            ['@authorizer', './apps/foam-authorizer/src'],
            ['@authorizer-test', './apps/foam-authorizer/src/test'],
          ],
          extensions: ['.ts', '.tsx', '.js', '.jsx', '.json'],
        },
      },
      react: {
        version: 'detect',
      },
    },
  },
  {
    rules: {
      'arrow-parens': 0,
      camelcase: 0,
      'comma-dangle': ['error', 'always-multiline'],
      'consistent-return': 0,
      'function-paren-newline': 0,
      'global-require': 0,
      'implicit-arrow-linebreak': 0,
      'import/no-cycle': 0,
      'no-console': ['off'],
      'no-extra-boolean-cast': 0,
      'no-nested-ternary': 0,
      'import/no-cycle': 'error',
      'no-return-assign': 0,
      'no-undef': ['warn'],
      'no-underscore-dangle': 0,
      'no-unused-expressions': 0,
      'no-fallthrough': 'error',
      '@typescript-eslint/no-unused-vars': ['error'],
      'object-curly-newline': 0,
      'object-curly-spacing': ['error', 'always'],
      'operator-linebreak': 0,
      'quote-props': 0,
      quotes: ['error', 'single', { avoidEscape: true }],
      semi: ['error', 'always'],
      'spaced-comment': 0,
      'no-console': 'warn',
      '@typescript-eslint/no-unsafe-assignment': 'off',
      '@typescript-eslint/no-unsafe-call': 'off',
      '@typescript-eslint/no-unsafe-member-access': 'off',
    },
  },
];
