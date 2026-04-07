import { readFileSync                     } from 'fs'
import { validateAction, validateWorkflow } from '@action-validator/core'

const actionFiles = [
  './action.yml',
  '.github/actions/setup-node/action.yml',
  '.github/actions/upload-pages/action.yml'
]

const workflowFiles = [
  '.github/workflows/lint-css.yml',
  '.github/workflows/lint-go.yml',
  '.github/workflows/lint-js.yml',
  '.github/workflows/pages.yml',
  '.github/workflows/test-go.yml',
  '.github/workflows/validate-actions-and-workflows.yml',
]

for (const actionFile of actionFiles) {
  const state = validateAction(readFileSync(actionFile, 'utf8'))

  if (state.errors.length > 0) {
    console.error(`${actionFile} is invalid:`, state.errors)
    process.exit(1)
  } else {
    console.log(`${actionFile} is valid`)
  }
}

for (const workflowFile of workflowFiles) {
  const state = validateWorkflow(readFileSync(workflowFile, 'utf8'))

  if (state.errors.length > 0) {
    console.error(`${workflowFile} is invalid:`, state.errors)
    process.exit(1)
  } else {
    console.log(`${workflowFile} is valid`)
  }
}
