"use client"
import React from 'react'
import useEnrollmentStore from '@/lib/store/enrollment-store'
import { ChevronRight } from 'lucide-react'
import { usePathname } from 'next/navigation'

// TODO: Change the server component when we use access_tokens (no longer rely on the session) to fetch data
export default function EnrollmentLayout({ children }: { children: React.ReactNode }) {
    const { serviceRequest } = useEnrollmentStore()
    const pathname = usePathname()

    const isFirstStep = pathname === '/enrollment/new'
    const isLastStep = pathname === '/enrollment/success'
    const service = serviceRequest?.code?.coding?.[0].display

    const breadcrumb = isFirstStep
        ? <span className='font-medium'>Verzoek controleren</span>
        : <a href={`${process.env.NEXT_PUBLIC_BASE_PATH || ""}/enrollment/new`} className="text-primary font-medium">Verzoek controleren</a>

    const title = service ? `${service} instellen` : "Instellen"

    return (
        <div className="w-full h-full">
            <div className="max-w-7xl px-5 mx-auto py-6">
                <nav className="flex items-center space-x-2 text-sm">
                    {!isLastStep && (
                        <>
                            {breadcrumb}
                            <ChevronRight className="h-4 w-4" />
                            <span className={`${isFirstStep && 'text-muted-foreground'}`}>{service}</span>
                        </>
                    )}
                </nav>
                <div className='text-2xl pt-2'>{isLastStep ? "Verzoek geaccepteerd" : isFirstStep ? "Verzoek controleren" : title}</div>
            </div>
            <div className="h-px bg-gray-200 mb-10 w-[calc(100%+3rem)] -mx-6"></div>
            <div className="max-w-7xl px-5 w-full mx-auto">
                {children}
            </div>
        </div >
    )
}
