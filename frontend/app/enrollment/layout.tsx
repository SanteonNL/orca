"use client"
import React from 'react'
import useEnrollmentStore from '@/lib/store/enrollment-store'
import { ChevronRight } from 'lucide-react'
import useTaskProgressStore from "@/lib/store/task-progress-store";

// TODO: Change the server component when we use access_tokens (no longer rely on the session) to fetch data
export default function EnrollmentLayout({ children }: { children: React.ReactNode }) {
    const { serviceRequest } = useEnrollmentStore()
    const { task } = useTaskProgressStore()

    const lastStepTaskStates = ['accepted', 'in-progress', 'rejected', 'failed', 'completed', 'cancelled', 'on-hold']
    const isFirstStep = task?.status == "requested"
    const isLastStep = task ? lastStepTaskStates.includes(task.status) : false
    const service = serviceRequest?.code?.coding?.[0].display
    const statusTitles = {
        "ready": service ? `${service} instellen` : "Instellen",
        "requested": service ? `${service} instellen` : "Instellen",
        "received": service ? `${service} instellen` : "Instellen",
        "accepted": "Verzoek geaccepteerd",
        "in-progress": "Verzoek in behandeling",
        "on-hold": "Uitvoering gepauzeerd",
        "completed": "Uitvoering afgerond",
        "cancelled": "Uitvoering geannuleerd",
        "failed": "Uitvoering mislukt",
        "rejected": "Verzoek afgewezen",
        "draft": "Verzoek in concept",
        "entered-in-error": "Verzoek gemarkeerd als foutief",
    }

    const isOverview = typeof window !== 'undefined' && window.location.pathname.endsWith('/list')
    const breadcrumb = isFirstStep
        ? <span className='font-medium'>Verzoek controleren</span>
        : <a href={`${process.env.NEXT_PUBLIC_BASE_PATH || ""}/enrollment/new`} className="text-primary font-medium">Verzoek controleren</a>

    // Determine title:
    // - If task is not null, use the status title based on task status
    // - If we're at the 'new' page ('/new'), set title to "Verzoek controleren"
    // - Otherwise, set title to 'Overzicht'
    const title = task ? statusTitles[task.status]
        : isOverview ? "Overzicht"
        : "Verzoek controleren"

    return (
        <div className="w-full h-full">
            <div className="max-w-7xl px-5 mx-auto py-6">
                {
                    isOverview ? <></> :
                    <nav className={`flex items-center space-x-2 text-sm ${isLastStep ? 'invisible' : 'inherit'}`}>
                        <>
                            {breadcrumb}
                            <ChevronRight className="h-4 w-4" />
                            <span className={`first-letter:uppercase ${isFirstStep ? 'text-muted-foreground' : ''}`}>{service}</span>
                        </>
                    </nav>
                }
                <div className='text-2xl pt-2 first-letter:uppercase'>{title}</div>
            </div>
            <div className="h-px bg-gray-200 mb-10"></div>
            <div className="max-w-7xl px-5 w-full mx-auto mb-5">
                {children}
            </div>
        </div>
    )
}
