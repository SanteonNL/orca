import React, { useEffect } from 'react'
import { Task } from 'fhir/r4'
import { getViewerData } from '@/app/actions'
import { Alert, Button } from '@mui/material'
import { LoaderIcon, RefreshCw } from 'lucide-react'

export default function DataViewer({ task }: { task: Task }) {

    const [data, setData] = React.useState<any[]>([])
    const [scpContext, setScpContext] = React.useState<string | null>(null) // used to determine the SCP context of an aggregate request
    const [error, setError] = React.useState<string | null>(null)

    useEffect(() => {
        console.log("Task changed")
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

        const taskSourceUrl = new URL(task.meta.source)
        taskSourceUrl.pathname = taskSourceUrl.pathname.replace(/\/Task\/[^/]+/, `/CarePlan/${carePlanId}`)
        setScpContext(taskSourceUrl.toString())
        setError(null)
    }, [task])

    useEffect(() => {
        fetchViewerData()
    }, [scpContext])

    const fetchViewerData = () => {

        if (!scpContext) return

        getViewerData(scpContext).then(({ data, error, message }) => {
            if (error) {
                setError(message)
                return
            } else {
                setData(data || [])
                setError(null)
            }
        })
    }

    return (

        <div className='w-full'>
            <h2>Data Viewer</h2>
            {error && <Alert severity="error">{error}</Alert>}
            <Button onClick={fetchViewerData}><RefreshCw /></Button>
            {data.length}
        </div>
    )
}
