import type { Resource } from 'fhir/r4'
import type { BundleParams, PaginationParams, ReadParams, SearchCallParams } from 'fhir-kit-client'

// fhir-kit-client v2 types every response as its own loose FhirResource, which can't be cast to the fhir/r4 types
// we use. These overloads restore the v1 typings (responses typed as any, fhir/r4 resources accepted as bodies).
declare module 'fhir-kit-client' {
    interface Client {
        read(params: ReadParams): Promise<any>
        search(params?: SearchCallParams): Promise<any>
        nextPage(params: PaginationParams): Promise<any> | undefined
        transaction(params: Omit<BundleParams, 'body'> & { body: Resource }): Promise<any>
    }
}
