import React from 'react'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import TaskOverviewTable from "@/app/enrollment/list/components/table";

export default function ListTasks() {
  return (
    <Card className="border-0 shadow-none px-0">
      <CardContent className="space-y-6 px-0">
        <TaskOverviewTable></TaskOverviewTable>
      </CardContent>
    </Card>
  )
}

