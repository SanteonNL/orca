import React from 'react'

export default function Navbar() {
    return (
        <nav className="bg-slate-100 text-zinc-800 border-gray-200 dark:bg-gray-900">
            <div className="max-w-screen-xl flex flex-wrap items-center justify-between mx-auto p-4">
                <a href="https://flowbite.com/" className="flex items-center space-x-3 rtl:space-x-reverse">
                    <span className="self-center text-2xl font-semibold whitespace-nowrap dark:text-white">Shared Care Planning</span>
                </a>
            </div>
        </nav>
    )
}
