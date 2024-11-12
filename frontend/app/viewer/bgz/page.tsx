import React from 'react'
import SwaggerUI from "swagger-ui-react"
import "swagger-ui-react/swagger-ui.css"


export default function BgzViewer() {
    return (
        <SwaggerUI url={`${process.env.NEXT_PUBLIC_BASE_PATH || ""}/bgz.json`} />
    )
}
