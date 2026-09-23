import { readFileSync,   globSync         } from 'node:fs'
import { validateAction, validateWorkflow } from '@action-validator/core'

const actions   = globSync(['action.yml', '.github/actions/*/action.yml'])
const workflows = globSync(['.github/workflows/*.yml'])

const validate = (files, validator) => {
  for (const file of files) {
    const { errors } = validator(readFileSync(file, 'utf8'))
    if (errors.length > 0) {
      console.error(`${file} is invalid:`, errors)
      process.exit(1)
    }
    console.log(`${file} is valid`)
  }
}

validate(actions,   validateAction)
validate(workflows, validateWorkflow)
