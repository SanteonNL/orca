import React, { useCallback, useEffect, JSX } from 'react'
import { Bundle, Task } from 'fhir/r4'
import { getAggregationUrl, getSupportContactLink } from '@/app/actions'
import { Alert } from '@mui/material'
import ObservationCard from './observation-card'
import NoObservationsFound from './no-observations-found'
import { Separator } from './ui/separator'
import { Spinner } from './spinner'
import { Button } from './ui/button'
import { RefreshCcw } from 'lucide-react'

export default function DataViewer({ task }: { task: Task }) {

    const [data, setData] = React.useState<any[]>([])
    const [loading, setLoading] = React.useState<boolean>(true)
    const [aggregationEndpoint, setAggegationEndpoint] = React.useState<string>()
    const [scpContext, setScpContext] = React.useState<string | null>(null) // used to determine the SCP context of an aggregate request
    const [error, setError] = React.useState<string | JSX.Element | null>(null)
    const [supportContactLink, setSupportContactLink] = React.useState<string>()

    useEffect(() => {
        Promise.all([getAggregationUrl(), getSupportContactLink()]).then(([url, link]) => {
            setAggegationEndpoint(url)
            setSupportContactLink(link)
        })
    }, [])

    const setSupportErrorMessage = useCallback((error: string) => {
        setError(<p>Helaas is er iets misgegaan bij het ophalen van de data. Indien het probleem aanhoudt, neem dan contact op met {supportContactLink ? <a href={supportContactLink} target='_blank' rel="noopener noreferrer">{supportContactLink}</a> : 'support'}. Vermeld hierbij de volgende foutmelding: {error}</p>)
    }, [supportContactLink]);


    const fetchObservations = useCallback(async () => {

        setLoading(true)

        if (!aggregationEndpoint) {
            setSupportErrorMessage('No aggregation endpoint found');
            setLoading(false)
            return
        }

        const requestUrl = `${aggregationEndpoint}/Observation/_search`;

        if (!scpContext) {
            setSupportErrorMessage('SCP context is not set for Task/' + task.id);
            setLoading(false)
            return;
        }

        try {
            const response = await fetch(requestUrl, {
                method: 'POST',
                headers: {
                    "X-Scp-Context": scpContext,
                    "Content-Type": "application/x-www-form-urlencoded",
                }
            })

            setLoading(false)

            if (!response.ok) {
                setSupportErrorMessage(`Failed to fetch ${requestUrl}: [${response.status}] ${response.statusText}`);
                return
            }

            const result = await response.json() as Bundle

            const entries = result.entry
                ?.filter((entry) => entry.resource?.resourceType === "Observation")
                ?.map((entry) => entry.resource)

            setData(entries || [])
            setError(null)
        } catch (e) {
            setLoading(false)
            setSupportErrorMessage(`Failed to fetch ${requestUrl}`);
        }
    }, [aggregationEndpoint, scpContext, setSupportErrorMessage, task.id])

    useEffect(() => {
        if (!task.basedOn) {
            setSupportErrorMessage(`Task (id=${task.id}) has no basedOn reference`)
            return
        }

        const taskWithCarePlanBasedOn = task.basedOn.find(ref => ref.reference?.includes("CarePlan/"))
        if (!taskWithCarePlanBasedOn) {
            setSupportErrorMessage(`Task (id=${task.id}) has no CarePlan as basedOn reference`)
            return
        }

        const carePlanId = taskWithCarePlanBasedOn.reference?.replace(/.*CarePlan\/([^?\/]+).*/, "$1")

        if (!carePlanId) {
            setSupportErrorMessage(`Unable to detect a CarePlan id for Task.basedOn (id=${task.id})`)
            return
        }

        if (!task.meta?.source) {
            setSupportErrorMessage(`Task.meta.source (id=${task.id}) is not defined, this is required to determine the SCP context`)
            return
        }

        try {
            const taskSourceUrl = new URL(task.meta.source)
            taskSourceUrl.pathname = taskSourceUrl.pathname.replace(/\/Task\/[^/]+/, `/CarePlan/${carePlanId}`)
            setScpContext(taskSourceUrl.toString())
            setError(null)
        } catch (e) {
            setSupportErrorMessage(`Task.meta.source (id=${task.id}) is not a valid URL: ${task.meta.source}`)
            return
        }
    }, [setSupportErrorMessage, task])

    useEffect(() => {
        fetchObservations()
    }, [aggregationEndpoint, fetchObservations, scpContext])

    return (
        <div>
            <Separator className='mt-5' />
            <div className="gap-3 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-y-4 mt-3 w-full">
                <div className='flex items-center justify-between col-span-full'>
                    <div>
                        <div className='font-bold text-xl'>Data viewer</div>
                        <div className='text-muted-foreground'>Hieronder wordt alle informatie getoond die in het kader van dit verzoek aangemaakt is door de betrokken partijen.</div>
                    </div>
                    <Button onClick={fetchObservations} className='float-right mt-3'><RefreshCcw /></Button>
                </div>
                {!loading && error && <Alert className='col-span-full' severity="error">{error}</Alert>}
                {loading && <div className='col-span-full'><Spinner /></div>}

                {!loading && !error &&
                    (
                        data.length ? (
                            data.filter((entry) => entry.resourceType === "Observation")
                                .map((observation) => (
                                    <ObservationCard key={observation.id} observation={observation} />
                                )))
                            :
                            <NoObservationsFound className='col-span-full' />
                    )}
            </div>
        </div>
    )
}