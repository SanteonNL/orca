import { Questionnaire, QuestionnaireResponse, Task } from "fhir/r4"

export const getQuestionnaireResponseId = (questionnaire?: Questionnaire) => {
    if (!questionnaire) throw new Error("Tried to generate a questionnaire response id but the Questionnaire is not defined")
    return `#questionnaire-response-${questionnaire.id}`
}

export const findQuestionnaire = (task?: Task) => {
    if (!task || !task.contained) return

    const questionnaires = task.contained.filter((contained) => contained.resourceType === "Questionnaire") as Questionnaire[]

    if (questionnaires.length < 1) console.warn("Found more than one Questionnaire for Task/" + task.id)

    return questionnaires.length ? questionnaires[0] : undefined
}

export const findQuestionnaireResponse = (task?: Task, questionnaire?: Questionnaire) => {
    if (!task || !task.contained || !questionnaire) return

    const expectedQuestionnaireId = getQuestionnaireResponseId(questionnaire)
    return task.contained.find((contained) => contained.id === expectedQuestionnaireId) as QuestionnaireResponse | undefined
}