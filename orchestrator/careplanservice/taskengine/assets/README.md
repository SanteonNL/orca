FHIR Questionnaire JSON files in this directory are used to negotiate FHIR workflow Tasks.
The `EmbeddedQuestionnaireLoader` takes the last part of the URL path, appends `.json` and then tries to load it from this directory.

For example, if the URL path is `/Questionnaire/MyQuestionnaire`, the loader will try to load `MyQuestionnaire.json` from this directory.