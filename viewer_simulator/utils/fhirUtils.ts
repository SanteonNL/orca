import { Questionnaire, QuestionnaireResponse, Task } from "fhir/r4"

export const fetchQuestionnaire = async (task?: Task) => {
    if (!task || !task.input) return

    const questionnaireRefs = task.input
        .filter((input) => input.valueReference?.reference?.startsWith("Questionnaire/"))
        .map((input) => input.valueReference?.reference)

    if (questionnaireRefs.length < 1) console.warn("Found more than one Questionnaire for Task/" + task.id)

    const questionnaireResp = await fetch(`${process.env.NEXT_PUBLIC_BASE_PATH || ""}/api/fhir/${questionnaireRefs[0]}`, {
        headers: {
            "Content-Type": "application/json"
        }
    })

    if (!questionnaireResp.ok) {
        throw new Error("Failed to fetch Questionnaire. Server message: " + questionnaireResp.statusText)
    }

    return await questionnaireResp.json() as Questionnaire
}

export const fetchQuestionnaireResponse = async (task?: Task, questionnaire?: Questionnaire) => {
    if (!task || !task.output || !questionnaire) return

    const questionnaireResponseRefs = task.output
        .filter((output) => output.valueReference?.reference?.startsWith("QuestionnaireResponse/"))
        .map((output) => output.valueReference?.reference)

    if (questionnaireResponseRefs.length < 1) console.warn("Found more than one Questionnaire for Task/" + task.id)

    const questionnaireResponseResp = await fetch(`${process.env.NEXT_PUBLIC_BASE_PATH || ""}/api/fhir/${questionnaireResponseRefs[0]}`, {
        headers: {
            "Content-Type": "application/json"
        }
    })

    if (!questionnaireResponseResp.ok) {
        throw new Error("Failed to fetch QuestionnaireResponse. Server message: " + questionnaireResponseResp.statusText)
    }

    return await questionnaireResponseResp.json() as QuestionnaireResponse
}