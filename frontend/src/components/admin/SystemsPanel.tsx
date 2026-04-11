import { useState, useRef, useCallback, useMemo } from "react";
import {
  Pencil,
  Trash2,
  Plus,
  ChevronRight,
  ChevronDown,
  GripVertical,
} from "lucide-react";
import {
  DndContext,
  closestCenter,
  type DragEndEvent,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  SortableContext,
  verticalListSortingStrategy,
  useSortable,
  arrayMove,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  useListSystemsQuery,
  useCreateSystemMutation,
  useUpdateSystemMutation,
  useDeleteSystemMutation,
  useListTalkgroupsQuery,
  useCreateTalkgroupMutation,
  useUpdateTalkgroupMutation,
  useDeleteTalkgroupMutation,
  useListUnitsQuery,
  useCreateUnitMutation,
  useUpdateUnitMutation,
  useDeleteUnitMutation,
  useListGroupsQuery,
  useListTagsQuery,
} from "@/app/slices/adminSlice";
import type { AdminSystem, AdminTalkgroup, AdminUnit } from "@/types";

// ─── Sortable system row ───

function SortableSystemRow({
  system,
  expanded,
  onToggle,
  onEdit,
  onDelete,
  onToggleAutoPopulate,
}: {
  system: AdminSystem;
  expanded: boolean;
  onToggle: () => void;
  onEdit: () => void;
  onDelete: () => void;
  onToggleAutoPopulate: () => void;
}) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: system.id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  return (
    <tr ref={setNodeRef} style={style}>
      <td className="w-8">
        <button
          className="btn btn-ghost btn-xs cursor-grab"
          {...attributes}
          {...listeners}
          aria-label="Drag to reorder"
        >
          <GripVertical className="w-4 h-4" />
        </button>
      </td>
      <td className="w-8">
        <button className="btn btn-ghost btn-xs" onClick={onToggle}>
          {expanded ? (
            <ChevronDown className="w-4 h-4" />
          ) : (
            <ChevronRight className="w-4 h-4" />
          )}
        </button>
      </td>
      <td>{system.systemId}</td>
      <td>{system.label}</td>
      <td>
        <input
          type="checkbox"
          className="toggle toggle-primary toggle-sm"
          checked={system.autoPopulate === 1}
          onChange={onToggleAutoPopulate}
        />
      </td>
      <td>{system.order}</td>
      <td className="flex gap-1">
        <button
          className="btn btn-ghost btn-xs"
          onClick={onEdit}
          aria-label="Edit system"
        >
          <Pencil className="w-4 h-4" />
        </button>
        <button
          className="btn btn-ghost btn-xs"
          onClick={onDelete}
          aria-label="Delete system"
        >
          <Trash2 className="w-4 h-4" />
        </button>
      </td>
    </tr>
  );
}

// ─── Virtualized talkgroup list ───

function TalkgroupList({
  talkgroups,
  onEdit,
  onDelete,
}: {
  talkgroups: AdminTalkgroup[];
  onEdit: (tg: AdminTalkgroup) => void;
  onDelete: (tg: AdminTalkgroup) => void;
}) {
  const parentRef = useRef<HTMLDivElement>(null);

  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: talkgroups.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 41,
    overscan: 10,
  });

  if (talkgroups.length === 0) {
    return <p className="text-sm opacity-60 py-2">No talkgroups</p>;
  }

  // For small lists, skip virtualization
  if (talkgroups.length <= 50) {
    return (
      <div className="overflow-x-auto">
        <table className="table table-zebra table-xs w-full">
          <thead>
            <tr>
              <th>TG ID</th>
              <th>Label</th>
              <th>Name</th>
              <th>Frequency</th>
              <th>Group</th>
              <th>Tag</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {talkgroups.map((tg) => (
              <tr key={tg.id}>
                <td>{tg.talkgroupId}</td>
                <td>{tg.label ?? "—"}</td>
                <td>{tg.name ?? "—"}</td>
                <td>
                  {tg.frequency != null
                    ? `${(tg.frequency / 1e6).toFixed(4)} MHz`
                    : "—"}
                </td>
                <td>{tg.groupId ?? "—"}</td>
                <td>{tg.tagId ?? "—"}</td>
                <td className="flex gap-1">
                  <button
                    className="btn btn-ghost btn-xs"
                    onClick={() => onEdit(tg)}
                    aria-label="Edit talkgroup"
                  >
                    <Pencil className="w-3 h-3" />
                  </button>
                  <button
                    className="btn btn-ghost btn-xs"
                    onClick={() => onDelete(tg)}
                    aria-label="Delete talkgroup"
                  >
                    <Trash2 className="w-3 h-3" />
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  }

  return (
    <div>
      <div className="overflow-x-auto">
        <table className="table table-zebra table-xs w-full">
          <thead>
            <tr>
              <th>TG ID</th>
              <th>Label</th>
              <th>Name</th>
              <th>Frequency</th>
              <th>Group</th>
              <th>Tag</th>
              <th>Actions</th>
            </tr>
          </thead>
        </table>
      </div>
      <div ref={parentRef} className="max-h-[400px] overflow-auto">
        <div
          style={{
            height: `${virtualizer.getTotalSize()}px`,
            position: "relative",
          }}
        >
          {virtualizer.getVirtualItems().map((virtualRow) => {
            const tg = talkgroups[virtualRow.index];
            return (
              <div
                key={tg.id}
                className="flex items-center text-xs border-b border-base-300"
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  height: `${virtualRow.size}px`,
                  transform: `translateY(${virtualRow.start}px)`,
                }}
              >
                <span className="w-[14%] px-2 truncate">{tg.talkgroupId}</span>
                <span className="w-[14%] px-2 truncate">{tg.label ?? "—"}</span>
                <span className="w-[14%] px-2 truncate">{tg.name ?? "—"}</span>
                <span className="w-[18%] px-2 truncate">
                  {tg.frequency != null
                    ? `${(tg.frequency / 1e6).toFixed(4)} MHz`
                    : "—"}
                </span>
                <span className="w-[10%] px-2 truncate">
                  {tg.groupId ?? "—"}
                </span>
                <span className="w-[10%] px-2 truncate">{tg.tagId ?? "—"}</span>
                <span className="w-[20%] px-2 flex gap-1">
                  <button
                    className="btn btn-ghost btn-xs"
                    onClick={() => onEdit(tg)}
                    aria-label="Edit talkgroup"
                  >
                    <Pencil className="w-3 h-3" />
                  </button>
                  <button
                    className="btn btn-ghost btn-xs"
                    onClick={() => onDelete(tg)}
                    aria-label="Delete talkgroup"
                  >
                    <Trash2 className="w-3 h-3" />
                  </button>
                </span>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

// ─── Main panel ───

const LED_COLORS = [
  "blue",
  "cyan",
  "green",
  "magenta",
  "red",
  "white",
  "yellow",
] as const;

interface SystemFormState {
  systemId: string;
  label: string;
  led: string;
  blacklists: string;
}

interface TgFormState {
  talkgroupId: string;
  label: string;
  name: string;
  frequency: string;
  led: string;
  groupId: string;
  tagId: string;
}

interface UnitFormState {
  unitId: string;
  label: string;
}

export default function SystemsPanel() {
  const { data: systems, isLoading: loadingSystems } = useListSystemsQuery();
  const { data: allTalkgroups } = useListTalkgroupsQuery();
  const { data: allUnits } = useListUnitsQuery();
  const { data: groups } = useListGroupsQuery();
  const { data: tags } = useListTagsQuery();

  const [createSystem] = useCreateSystemMutation();
  const [updateSystem] = useUpdateSystemMutation();
  const [deleteSystem] = useDeleteSystemMutation();
  const [createTalkgroup] = useCreateTalkgroupMutation();
  const [updateTalkgroup] = useUpdateTalkgroupMutation();
  const [deleteTalkgroup] = useDeleteTalkgroupMutation();
  const [createUnit] = useCreateUnitMutation();
  const [updateUnit] = useUpdateUnitMutation();
  const [deleteUnit] = useDeleteUnitMutation();

  const [expandedIds, setExpandedIds] = useState<Set<number>>(new Set());
  const [toast, setToast] = useState<string | null>(null);

  // System modal
  const [sysModalOpen, setSysModalOpen] = useState(false);
  const [editingSysId, setEditingSysId] = useState<number | null>(null);
  const [sysForm, setSysForm] = useState<SystemFormState>({
    systemId: "",
    label: "",
    led: "",
    blacklists: "",
  });

  // Talkgroup modal
  const [tgModalOpen, setTgModalOpen] = useState(false);
  const [editingTgId, setEditingTgId] = useState<number | null>(null);
  const [tgSystemId, setTgSystemId] = useState<number>(0);
  const [tgForm, setTgForm] = useState<TgFormState>({
    talkgroupId: "",
    label: "",
    name: "",
    frequency: "",
    led: "",
    groupId: "",
    tagId: "",
  });

  // Unit modal
  const [unitModalOpen, setUnitModalOpen] = useState(false);
  const [editingUnitId, setEditingUnitId] = useState<number | null>(null);
  const [unitSystemId, setUnitSystemId] = useState<number>(0);
  const [unitForm, setUnitForm] = useState<UnitFormState>({
    unitId: "",
    label: "",
  });

  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor),
  );

  const showError = useCallback((msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 5000);
  }, []);

  const sortedSystems = useMemo(
    () => (systems ? [...systems].sort((a, b) => a.order - b.order) : []),
    [systems],
  );

  const tgBySystem = useMemo(() => {
    const map = new Map<number, AdminTalkgroup[]>();
    if (allTalkgroups) {
      for (const tg of allTalkgroups) {
        const list = map.get(tg.systemId) ?? [];
        list.push(tg);
        map.set(tg.systemId, list);
      }
    }
    return map;
  }, [allTalkgroups]);

  const unitsBySystem = useMemo(() => {
    const map = new Map<number, AdminUnit[]>();
    if (allUnits) {
      for (const u of allUnits) {
        const list = map.get(u.systemId) ?? [];
        list.push(u);
        map.set(u.systemId, list);
      }
    }
    return map;
  }, [allUnits]);

  // ── Expand / collapse ──

  const toggleExpand = (id: number) => {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  // ── Drag to reorder ──

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id || !sortedSystems.length) return;

    const oldIndex = sortedSystems.findIndex((s) => s.id === active.id);
    const newIndex = sortedSystems.findIndex((s) => s.id === over.id);
    if (oldIndex === -1 || newIndex === -1) return;

    const reordered = arrayMove(sortedSystems, oldIndex, newIndex);

    // Update order for affected systems
    try {
      await Promise.all(
        reordered.map((sys, idx) => {
          if (sys.order !== idx) {
            return updateSystem({
              id: sys.id,
              systemId: sys.systemId,
              label: sys.label,
              autoPopulate: sys.autoPopulate,
              led: sys.led ?? null,
              blacklistsJson: sys.blacklistsJson ?? null,
              order: idx,
            }).unwrap();
          }
          return Promise.resolve();
        }),
      );
    } catch {
      showError("Failed to reorder systems");
    }
  };

  // ── System CRUD ──

  const openCreateSystem = () => {
    setEditingSysId(null);
    setSysForm({ systemId: "", label: "", led: "", blacklists: "" });
    setSysModalOpen(true);
  };

  const openEditSystem = (sys: AdminSystem) => {
    setEditingSysId(sys.id);
    setSysForm({
      systemId: String(sys.systemId),
      label: sys.label,
      led: sys.led ?? "",
      blacklists: sys.blacklistsJson
        ? (() => {
            try {
              const arr = JSON.parse(sys.blacklistsJson) as number[];
              return arr.join(",");
            } catch {
              return "";
            }
          })()
        : "",
    });
    setSysModalOpen(true);
  };

  const handleSystemSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    // Parse blacklists CSV → JSON array
    const blacklistIds = sysForm.blacklists
      .split(",")
      .map((s) => s.trim())
      .filter((s) => s !== "")
      .map(Number)
      .filter((n) => !isNaN(n));
    const blacklistsJson =
      blacklistIds.length > 0 ? JSON.stringify(blacklistIds) : null;
    try {
      if (editingSysId != null) {
        await updateSystem({
          id: editingSysId,
          systemId: Number(sysForm.systemId),
          label: sysForm.label,
          led: sysForm.led || null,
          blacklistsJson,
        }).unwrap();
      } else {
        await createSystem({
          systemId: Number(sysForm.systemId),
          label: sysForm.label,
          autoPopulate: 0,
          blacklistsJson,
          led: sysForm.led || null,
          order: sortedSystems.length,
        }).unwrap();
      }
      setSysModalOpen(false);
    } catch {
      showError(
        editingSysId ? "Failed to update system" : "Failed to create system",
      );
    }
  };

  const handleDeleteSystem = async (sys: AdminSystem) => {
    if (
      !window.confirm(
        `Delete system "${sys.label}" and all its talkgroups/units?`,
      )
    )
      return;
    try {
      await deleteSystem(sys.id).unwrap();
    } catch {
      showError("Failed to delete system");
    }
  };

  const handleToggleAutoPopulate = async (sys: AdminSystem) => {
    try {
      await updateSystem({
        id: sys.id,
        systemId: sys.systemId,
        label: sys.label,
        autoPopulate: sys.autoPopulate ? 0 : 1,
        led: sys.led ?? null,
        blacklistsJson: sys.blacklistsJson ?? null,
        order: sys.order,
      }).unwrap();
    } catch {
      showError("Failed to update system");
    }
  };

  // ── Talkgroup CRUD ──

  const openCreateTg = (systemId: number) => {
    setEditingTgId(null);
    setTgSystemId(systemId);
    setTgForm({
      talkgroupId: "",
      label: "",
      name: "",
      frequency: "",
      led: "",
      groupId: "",
      tagId: "",
    });
    setTgModalOpen(true);
  };

  const openEditTg = (tg: AdminTalkgroup) => {
    setEditingTgId(tg.id);
    setTgSystemId(tg.systemId);
    setTgForm({
      talkgroupId: String(tg.talkgroupId),
      label: tg.label ?? "",
      name: tg.name ?? "",
      frequency: tg.frequency != null ? String(tg.frequency) : "",
      led: tg.led ?? "",
      groupId: tg.groupId != null ? String(tg.groupId) : "",
      tagId: tg.tagId != null ? String(tg.tagId) : "",
    });
    setTgModalOpen(true);
  };

  const handleTgSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (editingTgId != null) {
        await updateTalkgroup({
          id: editingTgId,
          talkgroupId: Number(tgForm.talkgroupId),
          label: tgForm.label || null,
          name: tgForm.name || null,
          frequency: tgForm.frequency ? Number(tgForm.frequency) : null,
          led: tgForm.led || null,
          groupId: tgForm.groupId ? Number(tgForm.groupId) : null,
          tagId: tgForm.tagId ? Number(tgForm.tagId) : null,
        }).unwrap();
      } else {
        await createTalkgroup({
          systemId: tgSystemId,
          talkgroupId: Number(tgForm.talkgroupId),
          label: tgForm.label || null,
          name: tgForm.name || null,
          frequency: tgForm.frequency ? Number(tgForm.frequency) : null,
          led: tgForm.led || null,
          groupId: tgForm.groupId ? Number(tgForm.groupId) : null,
          tagId: tgForm.tagId ? Number(tgForm.tagId) : null,
          order: tgBySystem.get(tgSystemId)?.length ?? 0,
        }).unwrap();
      }
      setTgModalOpen(false);
    } catch {
      showError(
        editingTgId
          ? "Failed to update talkgroup"
          : "Failed to create talkgroup",
      );
    }
  };

  const handleDeleteTg = async (tg: AdminTalkgroup) => {
    if (!window.confirm(`Delete talkgroup ${tg.talkgroupId}?`)) return;
    try {
      await deleteTalkgroup(tg.id).unwrap();
    } catch {
      showError("Failed to delete talkgroup");
    }
  };

  // ── Unit CRUD ──

  const openCreateUnit = (systemId: number) => {
    setEditingUnitId(null);
    setUnitSystemId(systemId);
    setUnitForm({ unitId: "", label: "" });
    setUnitModalOpen(true);
  };

  const openEditUnit = (unit: AdminUnit) => {
    setEditingUnitId(unit.id);
    setUnitSystemId(unit.systemId);
    setUnitForm({
      unitId: String(unit.unitId),
      label: unit.label ?? "",
    });
    setUnitModalOpen(true);
  };

  const handleUnitSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (editingUnitId != null) {
        await updateUnit({
          id: editingUnitId,
          unitId: Number(unitForm.unitId),
          label: unitForm.label || null,
        }).unwrap();
      } else {
        await createUnit({
          systemId: unitSystemId,
          unitId: Number(unitForm.unitId),
          label: unitForm.label || null,
          order: unitsBySystem.get(unitSystemId)?.length ?? 0,
        }).unwrap();
      }
      setUnitModalOpen(false);
    } catch {
      showError(
        editingUnitId ? "Failed to update unit" : "Failed to create unit",
      );
    }
  };

  const handleDeleteUnit = async (unit: AdminUnit) => {
    if (!window.confirm(`Delete unit ${unit.unitId}?`)) return;
    try {
      await deleteUnit(unit.id).unwrap();
    } catch {
      showError("Failed to delete unit");
    }
  };

  if (loadingSystems) {
    return (
      <div className="flex justify-center py-12">
        <span className="loading loading-spinner loading-lg" />
      </div>
    );
  }

  return (
    <div>
      <h1 className="text-xl font-semibold mb-4">Systems</h1>
      <p className="text-sm text-base-content/70 mb-4">
        Define radio systems and their talkgroups. Systems represent a radio
        network (e.g. a county or agency). Each system contains talkgroups and
        units. Drag rows to reorder how they appear in the scanner.
      </p>
      <div className="card bg-base-200">
        <div className="card-body">
          <div className="overflow-x-auto">
            <DndContext
              sensors={sensors}
              collisionDetection={closestCenter}
              onDragEnd={handleDragEnd}
            >
              <SortableContext
                items={sortedSystems.map((s) => s.id)}
                strategy={verticalListSortingStrategy}
              >
                <table className="table table-zebra w-full">
                  <thead>
                    <tr>
                      <th className="w-8" />
                      <th className="w-8" />
                      <th>System ID</th>
                      <th>Label</th>
                      <th>Auto-populate</th>
                      <th>Order</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sortedSystems.map((sys) => (
                      <SystemRowGroup
                        key={sys.id}
                        system={sys}
                        expanded={expandedIds.has(sys.id)}
                        onToggle={() => toggleExpand(sys.id)}
                        onEdit={() => openEditSystem(sys)}
                        onDelete={() => handleDeleteSystem(sys)}
                        onToggleAutoPopulate={() =>
                          handleToggleAutoPopulate(sys)
                        }
                        talkgroups={tgBySystem.get(sys.id) ?? []}
                        units={unitsBySystem.get(sys.id) ?? []}
                        onEditTg={openEditTg}
                        onDeleteTg={handleDeleteTg}
                        onCreateTg={() => openCreateTg(sys.id)}
                        onEditUnit={openEditUnit}
                        onDeleteUnit={handleDeleteUnit}
                        onCreateUnit={() => openCreateUnit(sys.id)}
                      />
                    ))}
                    {sortedSystems.length === 0 && (
                      <tr>
                        <td colSpan={7} className="text-center opacity-60">
                          No systems found
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </SortableContext>
            </DndContext>
          </div>

          <div className="mt-4">
            <button className="btn btn-primary" onClick={openCreateSystem}>
              <Plus className="w-4 h-4" />
              Add System
            </button>
          </div>
        </div>
      </div>

      {/* System Modal */}
      <dialog className={`modal ${sysModalOpen ? "modal-open" : ""}`}>
        <div className="modal-box">
          <h3 className="font-bold text-lg mb-4">
            {editingSysId != null ? "Edit System" : "Create System"}
          </h3>
          <form onSubmit={handleSystemSubmit} className="flex flex-col gap-3">
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">System ID</span>
              </div>
              <input
                type="number"
                className="input input-bordered w-full"
                value={sysForm.systemId}
                onChange={(e) =>
                  setSysForm((p) => ({ ...p, systemId: e.target.value }))
                }
                required
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Label</span>
              </div>
              <input
                type="text"
                className="input input-bordered w-full"
                value={sysForm.label}
                onChange={(e) =>
                  setSysForm((p) => ({ ...p, label: e.target.value }))
                }
                required
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">LED Color</span>
                <span className="label-text-alt text-base-content/60">
                  Indicator color when playing audio from this system
                </span>
              </div>
              <select
                className="select select-bordered w-full"
                value={sysForm.led}
                onChange={(e) =>
                  setSysForm((p) => ({ ...p, led: e.target.value }))
                }
              >
                <option value="">Default (green)</option>
                {LED_COLORS.map((c) => (
                  <option key={c} value={c}>
                    {c.charAt(0).toUpperCase() + c.slice(1)}
                  </option>
                ))}
              </select>
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Blacklists</span>
                <span className="label-text-alt text-base-content/60">
                  Comma-separated talkgroup IDs to exclude when auto-populate is
                  on
                </span>
              </div>
              <textarea
                className="textarea textarea-bordered w-full"
                rows={2}
                placeholder="e.g. 1234,5678"
                value={sysForm.blacklists}
                onChange={(e) =>
                  setSysForm((p) => ({ ...p, blacklists: e.target.value }))
                }
              />
            </label>
            <div className="modal-action">
              <button
                type="button"
                className="btn"
                onClick={() => setSysModalOpen(false)}
              >
                Cancel
              </button>
              <button type="submit" className="btn btn-primary">
                {editingSysId != null ? "Save" : "Create"}
              </button>
            </div>
          </form>
        </div>
        <form method="dialog" className="modal-backdrop">
          <button type="button" onClick={() => setSysModalOpen(false)}>
            close
          </button>
        </form>
      </dialog>

      {/* Talkgroup Modal */}
      <dialog className={`modal ${tgModalOpen ? "modal-open" : ""}`}>
        <div className="modal-box">
          <h3 className="font-bold text-lg mb-4">
            {editingTgId != null ? "Edit Talkgroup" : "Add Talkgroup"}
          </h3>
          <form onSubmit={handleTgSubmit} className="flex flex-col gap-3">
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Talkgroup ID</span>
              </div>
              <input
                type="number"
                className="input input-bordered w-full"
                value={tgForm.talkgroupId}
                onChange={(e) =>
                  setTgForm((p) => ({ ...p, talkgroupId: e.target.value }))
                }
                required
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Label</span>
              </div>
              <input
                type="text"
                className="input input-bordered w-full"
                value={tgForm.label}
                onChange={(e) =>
                  setTgForm((p) => ({ ...p, label: e.target.value }))
                }
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Name</span>
              </div>
              <input
                type="text"
                className="input input-bordered w-full"
                value={tgForm.name}
                onChange={(e) =>
                  setTgForm((p) => ({ ...p, name: e.target.value }))
                }
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Group</span>
              </div>
              <select
                className="select select-bordered w-full"
                value={tgForm.groupId}
                onChange={(e) =>
                  setTgForm((p) => ({ ...p, groupId: e.target.value }))
                }
              >
                <option value="">— none —</option>
                {(groups ?? []).map((g) => (
                  <option key={g.id} value={String(g.id)}>
                    {g.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Tag</span>
              </div>
              <select
                className="select select-bordered w-full"
                value={tgForm.tagId}
                onChange={(e) =>
                  setTgForm((p) => ({ ...p, tagId: e.target.value }))
                }
              >
                <option value="">— none —</option>
                {(tags ?? []).map((t) => (
                  <option key={t.id} value={String(t.id)}>
                    {t.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">LED Color</span>
                <span className="label-text-alt text-base-content/60">
                  Overrides system color
                </span>
              </div>
              <select
                className="select select-bordered w-full"
                value={tgForm.led}
                onChange={(e) =>
                  setTgForm((p) => ({ ...p, led: e.target.value }))
                }
              >
                <option value="">Default (system color)</option>
                {LED_COLORS.map((c) => (
                  <option key={c} value={c}>
                    {c.charAt(0).toUpperCase() + c.slice(1)}
                  </option>
                ))}
              </select>
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Frequency (Hz)</span>
                <span className="label-text-alt text-base-content/60">
                  Display-only fake frequency
                </span>
              </div>
              <input
                type="number"
                className="input input-bordered w-full"
                value={tgForm.frequency}
                min={0}
                placeholder="e.g. 155325000"
                onChange={(e) =>
                  setTgForm((p) => ({ ...p, frequency: e.target.value }))
                }
              />
            </label>
            <div className="modal-action">
              <button
                type="button"
                className="btn"
                onClick={() => setTgModalOpen(false)}
              >
                Cancel
              </button>
              <button type="submit" className="btn btn-primary">
                {editingTgId != null ? "Save" : "Create"}
              </button>
            </div>
          </form>
        </div>
        <form method="dialog" className="modal-backdrop">
          <button type="button" onClick={() => setTgModalOpen(false)}>
            close
          </button>
        </form>
      </dialog>

      {/* Unit Modal */}
      <dialog className={`modal ${unitModalOpen ? "modal-open" : ""}`}>
        <div className="modal-box">
          <h3 className="font-bold text-lg mb-4">
            {editingUnitId != null ? "Edit Unit" : "Add Unit"}
          </h3>
          <form onSubmit={handleUnitSubmit} className="flex flex-col gap-3">
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Unit ID</span>
              </div>
              <input
                type="number"
                className="input input-bordered w-full"
                value={unitForm.unitId}
                onChange={(e) =>
                  setUnitForm((p) => ({ ...p, unitId: e.target.value }))
                }
                required
              />
            </label>
            <label className="form-control w-full">
              <div className="label">
                <span className="label-text">Label</span>
              </div>
              <input
                type="text"
                className="input input-bordered w-full"
                value={unitForm.label}
                onChange={(e) =>
                  setUnitForm((p) => ({ ...p, label: e.target.value }))
                }
              />
            </label>
            <div className="modal-action">
              <button
                type="button"
                className="btn"
                onClick={() => setUnitModalOpen(false)}
              >
                Cancel
              </button>
              <button type="submit" className="btn btn-primary">
                {editingUnitId != null ? "Save" : "Create"}
              </button>
            </div>
          </form>
        </div>
        <form method="dialog" className="modal-backdrop">
          <button type="button" onClick={() => setUnitModalOpen(false)}>
            close
          </button>
        </form>
      </dialog>

      {toast && (
        <div className="toast toast-end">
          <div className="alert alert-error">
            <span>{toast}</span>
          </div>
        </div>
      )}
    </div>
  );
}

// ─── SystemRowGroup: sortable row + expandable details ───

function SystemRowGroup({
  system,
  expanded,
  onToggle,
  onEdit,
  onDelete,
  onToggleAutoPopulate,
  talkgroups,
  units,
  onEditTg,
  onDeleteTg,
  onCreateTg,
  onEditUnit,
  onDeleteUnit,
  onCreateUnit,
}: {
  system: AdminSystem;
  expanded: boolean;
  onToggle: () => void;
  onEdit: () => void;
  onDelete: () => void;
  onToggleAutoPopulate: () => void;
  talkgroups: AdminTalkgroup[];
  units: AdminUnit[];
  onEditTg: (tg: AdminTalkgroup) => void;
  onDeleteTg: (tg: AdminTalkgroup) => void;
  onCreateTg: () => void;
  onEditUnit: (u: AdminUnit) => void;
  onDeleteUnit: (u: AdminUnit) => void;
  onCreateUnit: () => void;
}) {
  return (
    <>
      <SortableSystemRow
        system={system}
        expanded={expanded}
        onToggle={onToggle}
        onEdit={onEdit}
        onDelete={onDelete}
        onToggleAutoPopulate={onToggleAutoPopulate}
      />
      {expanded && (
        <tr>
          <td colSpan={7} className="bg-base-300 p-4">
            <div className="flex flex-col gap-6">
              {/* Talkgroups */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <h4 className="font-semibold text-sm">
                    Talkgroups ({talkgroups.length})
                  </h4>
                  <button
                    className="btn btn-primary btn-xs"
                    onClick={onCreateTg}
                  >
                    <Plus className="w-3 h-3" />
                    Add
                  </button>
                </div>
                <TalkgroupList
                  talkgroups={talkgroups}
                  onEdit={onEditTg}
                  onDelete={onDeleteTg}
                />
              </div>

              {/* Units */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <h4 className="font-semibold text-sm">
                    Units ({units.length})
                  </h4>
                  <button
                    className="btn btn-primary btn-xs"
                    onClick={onCreateUnit}
                  >
                    <Plus className="w-3 h-3" />
                    Add
                  </button>
                </div>
                {units.length === 0 ? (
                  <p className="text-sm opacity-60 py-2">No units</p>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="table table-zebra table-xs w-full">
                      <thead>
                        <tr>
                          <th>Unit ID</th>
                          <th>Label</th>
                          <th>Actions</th>
                        </tr>
                      </thead>
                      <tbody>
                        {units.map((u) => (
                          <tr key={u.id}>
                            <td>{u.unitId}</td>
                            <td>{u.label ?? "—"}</td>
                            <td className="flex gap-1">
                              <button
                                className="btn btn-ghost btn-xs"
                                onClick={() => onEditUnit(u)}
                                aria-label="Edit unit"
                              >
                                <Pencil className="w-3 h-3" />
                              </button>
                              <button
                                className="btn btn-ghost btn-xs"
                                onClick={() => onDeleteUnit(u)}
                                aria-label="Delete unit"
                              >
                                <Trash2 className="w-3 h-3" />
                              </button>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            </div>
          </td>
        </tr>
      )}
    </>
  );
}
