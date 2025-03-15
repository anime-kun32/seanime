import { PluginProvider, registry, RenderPluginComponents } from "@/app/(main)/_features/plugin/components/registry"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/components/ui/core/styling"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { PopoverAnatomy } from "@/components/ui/popover"
import { Tooltip } from "@/components/ui/tooltip"
import { getPixelsFromLength } from "@/lib/helpers/css"
import * as PopoverPrimitive from "@radix-ui/react-popover"
import { useAtomValue } from "jotai"
import Image from "next/image"
import React from "react"
import { LuCircleDashed } from "react-icons/lu"
import {
    Plugin_Server_TrayIconEventPayload,
    usePluginListenTrayCloseEvent,
    usePluginListenTrayIconEvent,
    usePluginListenTrayOpenEvent,
    usePluginListenTrayUpdatedEvent,
    usePluginSendRenderTrayEvent,
    usePluginSendTrayClickedEvent,
    usePluginSendTrayClosedEvent,
    usePluginSendTrayOpenedEvent,
} from "../generated/plugin-events"
import { __plugin_hasNavigatedAtom } from "./plugin-sidebar-tray"

/**
 * TrayIcon
 */
export type TrayIcon = {
    extensionId: string
} & Plugin_Server_TrayIconEventPayload

type TrayPluginProps = {
    trayIcon: TrayIcon
    place: "sidebar" | "top"
    width: number | null
}

export const PluginTrayContext = React.createContext<TrayPluginProps>({
    place: "sidebar",
    width: null,
    trayIcon: {
        extensionId: "",
        iconUrl: "",
        withContent: false,
        tooltipText: "",
        badgeNumber: 0,
        badgeIntent: "info",
        width: "30rem",
        minHeight: "auto",
    },
})

function PluginTrayProvider(props: { children: React.ReactNode, props: TrayPluginProps }) {
    return <PluginTrayContext.Provider value={props.props}>
        {props.children}
    </PluginTrayContext.Provider>
}

export function usePluginTray() {
    const context = React.useContext(PluginTrayContext)
    if (!context) {
        throw new Error("usePluginTray must be used within a PluginTrayProvider")
    }
    return context
}

export function PluginTray(props: TrayPluginProps) {

    const [open, setOpen] = React.useState(false)
    const [badgeNumber, setBadgeNumber] = React.useState(0)
    const [badgeIntent, setBadgeIntent] = React.useState("info")

    const { sendTrayOpenedEvent } = usePluginSendTrayOpenedEvent()
    const { sendTrayClosedEvent } = usePluginSendTrayClosedEvent()
    const { sendTrayClickedEvent } = usePluginSendTrayClickedEvent()

    const hasNavigated = useAtomValue(__plugin_hasNavigatedAtom)

    const firstRender = React.useRef(true)
    React.useEffect(() => {
        if (firstRender.current) {
            firstRender.current = false
            return
        }
        if (open) {
            sendTrayOpenedEvent({}, props.trayIcon.extensionId)
        } else {
            sendTrayClosedEvent({}, props.trayIcon.extensionId)
        }
    }, [open])

    usePluginListenTrayIconEvent((data) => {
        setBadgeNumber(data.badgeNumber)
        setBadgeIntent(data.badgeIntent)
    }, props.trayIcon.extensionId)

    usePluginListenTrayOpenEvent((data) => {
        if (hasNavigated) {
            setOpen(true)
        }
    }, props.trayIcon.extensionId)

    usePluginListenTrayCloseEvent((data) => {
        setOpen(false)
    }, props.trayIcon.extensionId)

    function handleClick() {
        sendTrayClickedEvent({}, props.trayIcon.extensionId)
    }


    const TrayIcon = () => {
        return (
            <div
                className="w-8 h-8 rounded-full flex items-center justify-center hover:bg-gray-800 cursor-pointer transition-all relative"
                onClick={handleClick}
            >
                <div className="w-8 h-8 rounded-full flex items-center justify-center overflow-hidden relative">
                    {props.trayIcon.iconUrl ? <Image
                        src={props.trayIcon.iconUrl}
                        alt="logo"
                        fill
                        className="p-1 w-full h-full object-contain"
                    /> : <div className="w-8 h-8 rounded-full flex items-center justify-center">
                        <LuCircleDashed className="text-2xl" />
                    </div>}
                </div>
                {!!badgeNumber && <Badge
                    intent={`${badgeIntent}-solid` as any}
                    size="sm"
                    className="absolute -top-2 -right-2 z-10 select-none pointer-events-none"
                >
                    {badgeNumber}
                </Badge>}
            </div>
        )
    }

    if (!props.trayIcon.withContent) {
        return <div className="cursor-pointer">
            {!!props.trayIcon.tooltipText ? <Tooltip
                side="right"
                trigger={<div>
                    <TrayIcon />
                </div>}
            >
                {props.trayIcon.tooltipText}
            </Tooltip> : <TrayIcon />}
        </div>
    }

    const designatedWidthPx = getPixelsFromLength(props.trayIcon.width || "30rem")
    const popoverWidth = (props.width && props.width < 1024)
        ? (designatedWidthPx >= props.width ? `calc(100vw - 30px)` : designatedWidthPx)
        : props.trayIcon.width || "30rem"

    console.log("popoverWidth", popoverWidth)
    console.log("designatedWidthPx", designatedWidthPx)
    console.log("props.width", props.width)

    return (
        <>
            <PopoverPrimitive.Root
                open={open}
                onOpenChange={setOpen}
                modal={false}
            >
                <PopoverPrimitive.Trigger
                    asChild
                >
                    <div>
                        {!!props.trayIcon.tooltipText ? <Tooltip
                            side={props.place === "sidebar" ? "right" : "bottom"}
                            trigger={<div>
                                <TrayIcon />
                            </div>}
                        >
                            {props.trayIcon.tooltipText}
                        </Tooltip> : <TrayIcon />}
                    </div>
                </PopoverPrimitive.Trigger>
                <PopoverPrimitive.Portal>
                    <PopoverPrimitive.Content
                        sideOffset={10}
                        side={props.place === "sidebar" ? "right" : "bottom"}
                        className={cn(PopoverAnatomy.root(), "bg-gray-950 p-0 shadow-xl rounded-xl mb-4")}
                        onOpenAutoFocus={(e) => e.preventDefault()}
                        style={{
                            width: popoverWidth,
                            minHeight: props.trayIcon.minHeight || "auto",
                            marginLeft: props.width && props.width < 1024 ? `10px` : undefined,
                        }}
                    >
                        <PluginTrayProvider props={props}>
                            <PluginTrayContent
                                open={open}
                                setOpen={setOpen}
                                {...props}
                            />
                        </PluginTrayProvider>
                    </PopoverPrimitive.Content>
                </PopoverPrimitive.Portal>
            </PopoverPrimitive.Root>
        </>
    )
}

type PluginTrayContentProps = {
    open: boolean
    setOpen: (open: boolean) => void
} & TrayPluginProps

function PluginTrayContent(props: PluginTrayContentProps) {
    const {
        trayIcon,
        open,
        setOpen,
    } = props

    const { sendRenderTrayEvent } = usePluginSendRenderTrayEvent()

    React.useEffect(() => {
        if (open) {
            sendRenderTrayEvent({}, trayIcon.extensionId)
        }
    }, [open])

    const [data, setData] = React.useState<any>(null)

    usePluginListenTrayUpdatedEvent((data) => {
        // console.log("tray:updated", extensionID, data)
        setData(data)
    }, trayIcon.extensionId)

    return (
        <div>
            {/*<p className="font-bold">*/}
            {/*    Extension name*/}
            {/*</p>*/}
            {/*<Separator className="my-2" />*/}

            <div
                className={cn(
                    "max-h-[35rem] overflow-y-auto p-3",
                )}
            >

                <PluginProvider registry={registry}>
                    {!!data && data.components ? <RenderPluginComponents data={data.components} /> : <LoadingSpinner />}
                </PluginProvider>

            </div>
        </div>
    )
}
