import React, { useCallback, useEffect } from 'react'
import { Bundle, Task } from 'fhir/r4'
import { getAggregationUrl } from '@/app/actions'
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
    const [aggregationEndpoint, setAggegationEndpoint] = React.useState<string | undefined>()
    const [scpContext, setScpContext] = React.useState<string | null>(null) // used to determine the SCP context of an aggregate request
    const [error, setError] = React.useState<string | null>(null)

    useEffect(() => {
        getAggregationUrl().then((url) => {
            setAggegationEndpoint(url)
        })
    }, [])


    const fetchObservations = useCallback(async () => {

        setLoading(true)

        if (!aggregationEndpoint) {
            setError('No aggregation endpoint found');
            setLoading(false)
            return
        }

        const requestUrl = `${aggregationEndpoint}/Observation/_search`;

        if (!scpContext) {
            setError('SCP context is not set');
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
                setError(`Failed to fetch ${requestUrl}: ${response.statusText}`);
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
            setError(`Failed to fetch ${requestUrl}`);
        }
    }, [aggregationEndpoint, scpContext])

    useEffect(() => {
        if (!task.basedOn) {
            setError('Task has no basedOn reference')
            return
        }

        const taskWithCarePlanBasedOn = task.basedOn.find(ref => ref.reference?.includes("CarePlan/"))
        if (!taskWithCarePlanBasedOn) {
            setError('Task has no CarePlan as basedOn reference')
            return
        }

        const carePlanId = taskWithCarePlanBasedOn.reference?.replace(/.*CarePlan\/([^?\/]+).*/, "$1")

        if (!carePlanId) {
            setError(`Unable to detect a CarePlan id for Task.basedOn (Task/${task.id})`)
            return
        }

        if (!task.meta?.source) {
            setError(`Task.meta.source is not defined, this is required to determine the SCP context`)
            return
        }

        try {
            const taskSourceUrl = new URL(task.meta.source)
            taskSourceUrl.pathname = taskSourceUrl.pathname.replace(/\/Task\/[^/]+/, `/CarePlan/${carePlanId}`)
            setScpContext(taskSourceUrl.toString())
            setError(null)
        } catch (e) {
            setError(`Task.meta.source is not a valid URL: ${task.meta.source}`)
            return
        }
    }, [task])

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