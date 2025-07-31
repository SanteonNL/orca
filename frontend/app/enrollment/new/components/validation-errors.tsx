import { OperationOutcomeIssue } from 'fhir/r4'

interface ValidationErrorsProps {
    validationErrors: OperationOutcomeIssue[]
}

export default function ValidationErrors({ validationErrors }: ValidationErrorsProps) {
    return (
        <div className='text-red-500 mb-2'>
            <h3 className='font-semibold'>Validation Errors:</h3>
            {validationErrors.every(m => !m.diagnostics) ? ('An unknown error occurred') :
                (
                    validationErrors?.filter(issue => issue.diagnostics).map((issue, index) => (
                        <p key={index}>
                            {issue.diagnostics || "Unknown error"}
                        </p>
                    ))
                )
            }
        </div>
    )
}
