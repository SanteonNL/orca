"use client"
import { useParams } from 'next/navigation'
import React from 'react'

export default function TaskEnrollmentView() {

    const { taskId } = useParams()

    return (
        <div>
            <p>Show subtasks or status for Task/{taskId}</p>
            <p>TODO: Implement under INT-211</p>
        </div>
    )
}
