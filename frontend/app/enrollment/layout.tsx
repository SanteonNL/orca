"use client"
import React from 'react'

export default function EnrollmentLayout({children}: { children: React.ReactNode }) {
    return (
        <div className="w-full h-full">
            {children}
        </div>
    )
}