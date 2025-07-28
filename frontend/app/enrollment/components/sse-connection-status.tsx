"use client"
import { HoverCard, HoverCardContent, HoverCardTrigger } from '@/components/ui/hover-card'
import useTaskProgressStore from '@/lib/store/task-progress-store'
import { AlertCircle, CheckCircle2 } from 'lucide-react'
import React from 'react'

export default function TaskSseConnectionStatus() {

    const { eventSourceConnected } = useTaskProgressStore()

    return (
        <HoverCard>
            <HoverCardTrigger asChild>
                <div className={`fixed bottom-5 right-5 h-3 w-3 rounded-full ${eventSourceConnected ? "bg-green-500" : "bg-red-500"}`} />
            </HoverCardTrigger>
            <HoverCardContent className="w-120">
                <div className="flex flex-col space-y-2 p-4">
                    <div className="flex items-center justify-between">
                        <h3 className="text-lg font-medium">Real-time updates</h3>
                        <div className="flex items-center space-x-2">
                            <div data-testid="sse-connection" className={`h-3 w-3 rounded-full ${eventSourceConnected ? "bg-green-500" : "bg-red-500"}`} />
                            <span className="text-sm font-medium capitalize">{eventSourceConnected ? 'Verbonden' : 'Verbinding verbroken'}</span>
                        </div>
                    </div>

                    <div className="flex items-center text-sm text-muted-foreground">
                        {eventSourceConnected ? (
                            <CheckCircle2 className="mr-2 h-4 w-4 text-green-500" />
                        ) : (
                            <AlertCircle className="mr-2 h-4 w-4 text-red-500" />
                        )}
                        <span>
                            {eventSourceConnected
                                ? "Successvol verbonden - U ontvangt real-time updates"
                                : "Verbinding verbroken - U ontvangt geen real-time updates"}
                        </span>
                    </div>
                </div>
            </HoverCardContent>
        </HoverCard>
    )
}
