import React from "react";

export default function TaskBody({children}: { children: React.ReactNode }) {
    return (
        <div className="max-w-7xl px-5 w-full mx-auto mb-5">
            {children}
        </div>
    )
}