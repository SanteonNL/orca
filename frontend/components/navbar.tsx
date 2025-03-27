import React from 'react'

export default function Navbar() {
    return (
        <nav className="bg-primary text-white border-gray-200">
            <div className="max-w-(--breakpoint-xl) flex flex-wrap items-center justify-between mx-auto p-4">
                <a href="#" className="flex items-center space-x-3 rtl:space-x-reverse">
                    <span className="self-center text-2xl font-semibold whitespace-nowrap dark:text-white">Shared Care Planning</span>
                </a>
            </div>
        </nav>
    )
}
