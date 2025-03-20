import { Resource, SearchParameter } from "fhir/r4";

export const supportedResourceTypes = ['Task', 'Condition', 'Observation'];
export const resources: { [key: string]: Resource[] } = {
    Task: [],
    Condition: [],
    Observation: []
};

//expression MUST be json path
export const searchParameters: SearchParameter[] = [{
    resourceType: 'SearchParameter',
    id: 'based-on',
    name: 'based-on',
    code: 'basedOn',
    type: 'reference',
    expression: '$.basedOn[*].identifier',
    base: [],
    description: '',
    status: 'active',
    url: ''
}];