import neostandard from 'neostandard'

export default [
  ...neostandard({
    env: ['browser'],
  }),
  {
    rules: {
      '@stylistic/no-multi-spaces':         'off',
      '@stylistic/key-spacing':             'off',
      '@stylistic/object-property-newline': 'off',
      '@stylistic/space-in-parens':         'off',
    }
  }
]
