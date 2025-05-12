import {Endpoint, Bundle, Identifier} from 'fhir/r4';
import {getPatientViewerTestUrl} from "@/app/actions";
import {createScpClient} from "@/lib/fhirUtils";

export async function getLaunchableApps(organization: Identifier) {
    const testAppURL = await getPatientViewerTestUrl();
    if (testAppURL) {
        return {
            Name: "Test App",
            URL: testAppURL,
        }
    }
    let endpoints = await createScpClient().search({
        resourceType: "Endpoint",
        headers: {
            "Cache-Control": "no-cache",
            "X-Scp-Entity-Identifier": `${organization.system}|${organization.value}`,
        }
    }) as Bundle<Endpoint>;
    // filter endpoints, only show endpoints that:
    // - have status 'active';
    // - have connection type `http://santeonnl.github.io/shared-care-planning/endpoint-connection-type|web-oauth2`
    endpoints = {
        ...endpoints,
        entry: endpoints.entry?.filter((entry) => {
            if (!entry.resource) {
                return false
            }
            const endpoint = entry?.resource as Endpoint;
            return endpoint.status == "active"
                && endpoint.connectionType?.system == "http://santeonnl.github.io/shared-care-planning/endpoint-connection-type"
                && endpoint.connectionType?.code == "web-oauth2";
        }),
    }
    // if no endpoints found, return undefined
    if (endpoints.entry?.length == 0) {
        return undefined
    }
    if (endpoints.entry?.length != 1) {
        // might want to support a list in the future
        console.warn("More than one launchable application found for organization", organization, endpoints.entry);
    }
    return {
        Name: endpoints.entry?.[0].resource?.name,
        URL: endpoints.entry?.[0].resource?.address,
    }
}