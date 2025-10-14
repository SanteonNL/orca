import React from "react";


export default function TaskHeading({children, title}: { children?: React.ReactNode, title: string }) {
    return (
        <>
            <div className="max-w-7xl px-5 mx-auto py-6">
                {children}
                <div className='text-2xl pt-2 first-letter:uppercase'>{title}</div>
            </div>
            <div className="h-px bg-gray-200 mb-10"></div>
        </>
    )
}